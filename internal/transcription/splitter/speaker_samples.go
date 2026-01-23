package splitter

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"scriberr/internal/transcription/interfaces"
	"scriberr/pkg/logger"
)

const (
	MinSampleDurationSec = 2.0
	MaxSampleDurationSec = 10.0
)

// SpeakerSample represents an extracted audio sample for a speaker
type SpeakerSample struct {
	Speaker    string  // Speaker label (A, B, C)
	StartTime  float64 // Start time in source audio
	EndTime    float64 // End time in source audio
	FilePath   string  // Path to extracted audio clip
	Base64Data string  // Base64 encoded audio for API (data URL format)
}

// SpeakerReference is the API format for known_speaker_references
type SpeakerReference struct {
	Speaker        string `json:"speaker"`
	ReferenceAudio string `json:"reference_audio"`
}

// ExtractSpeakerSamples extracts audio samples for each speaker from transcription result
func ExtractSpeakerSamples(ctx context.Context, result *interfaces.TranscriptResult, audioPath string, tempDir string) ([]SpeakerSample, error) {
	if result == nil || len(result.Segments) == 0 {
		return nil, nil
	}

	speakerSegments := groupSegmentsBySpeaker(result.Segments)
	if len(speakerSegments) == 0 {
		logger.Debug("No speaker segments found for extraction")
		return nil, nil
	}

	sampleDir := filepath.Join(tempDir, "speaker_samples")
	if err := os.MkdirAll(sampleDir, 0755); err != nil {
		return nil, fmt.Errorf("create sample directory: %w", err)
	}

	var samples []SpeakerSample
	for speaker, segments := range speakerSegments {
		sample, err := extractBestSample(ctx, speaker, segments, audioPath, sampleDir)
		if err != nil {
			logger.Warn("Failed to extract sample for speaker", "speaker", speaker, "error", err)
			continue
		}
		if sample != nil {
			samples = append(samples, *sample)
		}
	}

	logger.Info("Extracted speaker samples", "count", len(samples), "speakers", len(speakerSegments))
	return samples, nil
}

// ToSpeakerReferences converts samples to API format
func ToSpeakerReferences(samples []SpeakerSample) []SpeakerReference {
	refs := make([]SpeakerReference, len(samples))
	for i, s := range samples {
		refs[i] = SpeakerReference{
			Speaker:        s.Speaker,
			ReferenceAudio: s.Base64Data,
		}
	}
	return refs
}

// groupSegmentsBySpeaker groups transcript segments by speaker label
func groupSegmentsBySpeaker(segments []interfaces.TranscriptSegment) map[string][]interfaces.TranscriptSegment {
	result := make(map[string][]interfaces.TranscriptSegment)
	for _, seg := range segments {
		if seg.Speaker == nil || *seg.Speaker == "" {
			continue
		}
		speaker := *seg.Speaker
		result[speaker] = append(result[speaker], seg)
	}
	return result
}

// extractBestSample finds and extracts the best audio segment for a speaker
func extractBestSample(ctx context.Context, speaker string, segments []interfaces.TranscriptSegment, audioPath string, outputDir string) (*SpeakerSample, error) {
	seg := selectBestSegment(segments)
	if seg == nil {
		return nil, nil
	}

	duration := seg.End - seg.Start
	outputPath := filepath.Join(outputDir, fmt.Sprintf("speaker_%s.mp3", speaker))

	if err := extractAudioSegment(ctx, audioPath, seg.Start, duration, outputPath); err != nil {
		return nil, fmt.Errorf("extract audio: %w", err)
	}

	base64Data, err := encodeAsDataURL(outputPath)
	if err != nil {
		return nil, fmt.Errorf("encode base64: %w", err)
	}

	return &SpeakerSample{
		Speaker:    speaker,
		StartTime:  seg.Start,
		EndTime:    seg.End,
		FilePath:   outputPath,
		Base64Data: base64Data,
	}, nil
}

// selectBestSegment selects the best segment for speaker sample extraction
func selectBestSegment(segments []interfaces.TranscriptSegment) *interfaces.TranscriptSegment {
	if len(segments) == 0 {
		return nil
	}

	// Sort by duration descending
	sorted := make([]interfaces.TranscriptSegment, len(segments))
	copy(sorted, segments)
	sort.Slice(sorted, func(i, j int) bool {
		return (sorted[i].End - sorted[i].Start) > (sorted[j].End - sorted[j].Start)
	})

	// Find first segment within duration bounds
	for i := range sorted {
		duration := sorted[i].End - sorted[i].Start
		if duration >= MinSampleDurationSec && duration <= MaxSampleDurationSec {
			return &sorted[i]
		}
	}

	// No ideal segment found - use longest available if it meets minimum
	if duration := sorted[0].End - sorted[0].Start; duration >= MinSampleDurationSec {
		seg := sorted[0]
		// Trim to max duration if needed
		if duration > MaxSampleDurationSec {
			seg.End = seg.Start + MaxSampleDurationSec
		}
		return &seg
	}

	// Try to concatenate consecutive segments
	return concatenateSegments(segments)
}

// concatenateSegments attempts to find consecutive segments that together meet minimum duration
func concatenateSegments(segments []interfaces.TranscriptSegment) *interfaces.TranscriptSegment {
	if len(segments) == 0 {
		return nil
	}

	// Sort by start time
	sorted := make([]interfaces.TranscriptSegment, len(segments))
	copy(sorted, segments)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Start < sorted[j].Start
	})

	// Find consecutive segments
	var start float64 = sorted[0].Start
	var end float64 = sorted[0].End

	for i := 1; i < len(sorted); i++ {
		gap := sorted[i].Start - end
		// Allow up to 1 second gap between segments
		if gap <= 1.0 {
			end = sorted[i].End
			if end-start >= MinSampleDurationSec {
				// Trim to max duration
				if end-start > MaxSampleDurationSec {
					end = start + MaxSampleDurationSec
				}
				return &interfaces.TranscriptSegment{
					Start:   start,
					End:     end,
					Speaker: sorted[0].Speaker,
				}
			}
		} else {
			// Reset and try from this segment
			start = sorted[i].Start
			end = sorted[i].End
		}
	}

	// Return whatever we have if it meets minimum
	if end-start >= MinSampleDurationSec {
		if end-start > MaxSampleDurationSec {
			end = start + MaxSampleDurationSec
		}
		return &interfaces.TranscriptSegment{
			Start:   start,
			End:     end,
			Speaker: sorted[0].Speaker,
		}
	}

	return nil
}

// extractAudioSegment uses ffmpeg to extract an audio segment
func extractAudioSegment(ctx context.Context, inputPath string, startTime, duration float64, outputPath string) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", startTime),
		"-t", fmt.Sprintf("%.3f", duration),
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "libmp3lame",
		"-b:a", "64k",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("FFmpeg extraction failed", "error", err, "output", string(output))
		return fmt.Errorf("ffmpeg: %w", err)
	}

	return nil
}

// encodeAsDataURL encodes an audio file as a base64 data URL
func encodeAsDataURL(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:audio/mp3;base64,%s", encoded), nil
}

// CleanupSpeakerSamples removes extracted sample files
func CleanupSpeakerSamples(samples []SpeakerSample) {
	for _, s := range samples {
		if s.FilePath != "" {
			if err := os.Remove(s.FilePath); err != nil {
				logger.Debug("Failed to cleanup speaker sample", "path", s.FilePath, "error", err)
			}
		}
	}
}
