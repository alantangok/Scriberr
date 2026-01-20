package splitter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"scriberr/internal/transcription/interfaces"
	"scriberr/pkg/logger"
)

// Note: MergeResults function is in merger.go

const (
	// MaxFileSizeBytes is the maximum file size before splitting (25MB)
	MaxFileSizeBytes = 25 * 1024 * 1024
	// MaxDurationMinutes is the maximum duration before splitting (25 minutes)
	MaxDurationMinutes = 25
	// ChunkDurationMinutes is the target chunk duration (10 minutes for safety margin)
	ChunkDurationMinutes = 10
)

// AudioSplitter handles splitting large audio files into chunks
type AudioSplitter struct {
	tempDirectory string
}

// NewAudioSplitter creates a new audio splitter
func NewAudioSplitter(tempDir string) *AudioSplitter {
	return &AudioSplitter{
		tempDirectory: tempDir,
	}
}

// ChunkInfo contains information about a split audio chunk
type ChunkInfo struct {
	FilePath      string
	StartTime     float64 // Start time in seconds relative to original
	Duration      float64 // Duration in seconds
	OriginalIndex int     // Index in the chunk sequence
}

// SplitResult contains the result of splitting an audio file
type SplitResult struct {
	Chunks       []ChunkInfo
	OriginalPath string
	NeedsSplit   bool
}

// NeedsSplitting checks if an audio file needs to be split
func (s *AudioSplitter) NeedsSplitting(input interfaces.AudioInput) bool {
	// Check file size (25MB limit)
	if input.Size > MaxFileSizeBytes {
		logger.Info("Audio file exceeds size limit",
			"size_mb", float64(input.Size)/(1024*1024),
			"limit_mb", float64(MaxFileSizeBytes)/(1024*1024))
		return true
	}

	// Check duration (25 minutes limit)
	durationMinutes := input.Duration.Minutes()
	if durationMinutes > MaxDurationMinutes {
		logger.Info("Audio file exceeds duration limit",
			"duration_min", durationMinutes,
			"limit_min", MaxDurationMinutes)
		return true
	}

	return false
}

// Split splits an audio file into chunks using ffmpeg
func (s *AudioSplitter) Split(ctx context.Context, input interfaces.AudioInput, jobID string) (*SplitResult, error) {
	if !s.NeedsSplitting(input) {
		return &SplitResult{
			Chunks: []ChunkInfo{{
				FilePath:      input.FilePath,
				StartTime:     0,
				Duration:      input.Duration.Seconds(),
				OriginalIndex: 0,
			}},
			OriginalPath: input.FilePath,
			NeedsSplit:   false,
		}, nil
	}

	logger.Info("Splitting audio file",
		"file", input.FilePath,
		"size_mb", float64(input.Size)/(1024*1024),
		"duration_min", input.Duration.Minutes())

	// Create chunk directory
	chunkDir := filepath.Join(s.tempDirectory, jobID, "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunk directory: %w", err)
	}

	// Calculate chunk duration based on file characteristics
	chunkDurationSec := s.calculateChunkDuration(input)

	// Get file extension for output
	ext := filepath.Ext(input.FilePath)
	if ext == "" {
		ext = ".mp3" // Default to mp3
	}

	// Build ffmpeg command for segmentation
	outputPattern := filepath.Join(chunkDir, fmt.Sprintf("chunk_%%03d%s", ext))

	args := []string{
		"-i", input.FilePath,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%.0f", chunkDurationSec),
		"-c", "copy", // Copy without re-encoding for speed
		"-reset_timestamps", "1",
		"-map", "0:a", // Only audio stream
		outputPattern,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("FFmpeg split failed", "error", err, "output", string(output))
		return nil, fmt.Errorf("ffmpeg split failed: %w", err)
	}

	// Find all generated chunks
	chunks, err := s.findChunks(chunkDir, ext)
	if err != nil {
		return nil, fmt.Errorf("failed to find chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks were created")
	}

	// Get duration for each chunk using ffprobe
	if err := s.populateChunkDurations(ctx, chunks); err != nil {
		logger.Warn("Failed to get chunk durations, estimating", "error", err)
		s.estimateChunkDurations(chunks, input.Duration.Seconds(), chunkDurationSec)
	}

	logger.Info("Audio split complete",
		"chunks", len(chunks),
		"chunk_duration_sec", chunkDurationSec)

	return &SplitResult{
		Chunks:       chunks,
		OriginalPath: input.FilePath,
		NeedsSplit:   true,
	}, nil
}

// calculateChunkDuration determines optimal chunk duration
func (s *AudioSplitter) calculateChunkDuration(input interfaces.AudioInput) float64 {
	// Default to 10 minutes (600 seconds)
	chunkDuration := float64(ChunkDurationMinutes * 60)

	// If we have bitrate info, calculate based on target size
	if bitrateStr, ok := input.Metadata["bitrate"]; ok {
		if bitrate, err := strconv.ParseFloat(bitrateStr, 64); err == nil && bitrate > 0 {
			// Target 20MB per chunk (safe margin under 25MB)
			targetSizeBytes := float64(20 * 1024 * 1024)
			bytesPerSecond := bitrate / 8
			calculatedDuration := targetSizeBytes / bytesPerSecond

			// Use the smaller of calculated vs default
			if calculatedDuration < chunkDuration {
				chunkDuration = calculatedDuration
			}
		}
	}

	// Minimum 5 minutes, maximum 15 minutes
	if chunkDuration < 300 {
		chunkDuration = 300
	}
	if chunkDuration > 900 {
		chunkDuration = 900
	}

	return chunkDuration
}

// findChunks finds all chunk files in the directory
func (s *AudioSplitter) findChunks(chunkDir, ext string) ([]ChunkInfo, error) {
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return nil, err
	}

	var chunks []ChunkInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ext) {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "chunk_") {
			continue
		}

		// Extract index from filename (chunk_000.mp3 -> 0)
		name := strings.TrimSuffix(entry.Name(), ext)
		indexStr := strings.TrimPrefix(name, "chunk_")
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			continue
		}

		chunks = append(chunks, ChunkInfo{
			FilePath:      filepath.Join(chunkDir, entry.Name()),
			OriginalIndex: index,
		})
	}

	// Sort by index
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].OriginalIndex < chunks[j].OriginalIndex
	})

	return chunks, nil
}

// populateChunkDurations gets actual duration for each chunk using ffprobe
func (s *AudioSplitter) populateChunkDurations(ctx context.Context, chunks []ChunkInfo) error {
	var cumulativeStart float64

	for i := range chunks {
		duration, err := s.getAudioDuration(ctx, chunks[i].FilePath)
		if err != nil {
			return err
		}

		chunks[i].StartTime = cumulativeStart
		chunks[i].Duration = duration
		cumulativeStart += duration
	}

	return nil
}

// estimateChunkDurations estimates durations when ffprobe fails
func (s *AudioSplitter) estimateChunkDurations(chunks []ChunkInfo, totalDuration, chunkDuration float64) {
	var cumulativeStart float64

	for i := range chunks {
		chunks[i].StartTime = cumulativeStart

		// Last chunk may be shorter
		remaining := totalDuration - cumulativeStart
		if remaining < chunkDuration {
			chunks[i].Duration = remaining
		} else {
			chunks[i].Duration = chunkDuration
		}

		cumulativeStart += chunks[i].Duration
	}
}

// getAudioDuration gets the duration of an audio file using ffprobe
func (s *AudioSplitter) getAudioDuration(ctx context.Context, filePath string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// CleanupChunks removes all chunk files
func (s *AudioSplitter) CleanupChunks(result *SplitResult) {
	if result == nil || !result.NeedsSplit {
		return
	}

	for _, chunk := range result.Chunks {
		if chunk.FilePath != result.OriginalPath {
			if err := os.Remove(chunk.FilePath); err != nil {
				logger.Warn("Failed to cleanup chunk", "file", chunk.FilePath, "error", err)
			}
		}
	}

	// Try to remove the chunk directory
	if len(result.Chunks) > 0 {
		chunkDir := filepath.Dir(result.Chunks[0].FilePath)
		_ = os.Remove(chunkDir) // Ignore error if not empty
	}
}
