package splitter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scriberr/internal/transcription/interfaces"
)

func TestGroupSegmentsBySpeaker(t *testing.T) {
	speakerA := "A"
	speakerB := "B"
	empty := ""

	tests := []struct {
		name     string
		segments []interfaces.TranscriptSegment
		want     map[string]int // speaker -> count
	}{
		{
			name:     "empty segments",
			segments: nil,
			want:     map[string]int{},
		},
		{
			name: "no speakers",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "hello"},
				{Start: 1, End: 2, Text: "world"},
			},
			want: map[string]int{},
		},
		{
			name: "single speaker",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "hello", Speaker: &speakerA},
				{Start: 1, End: 2, Text: "world", Speaker: &speakerA},
			},
			want: map[string]int{"A": 2},
		},
		{
			name: "multiple speakers",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "hello", Speaker: &speakerA},
				{Start: 1, End: 2, Text: "hi", Speaker: &speakerB},
				{Start: 2, End: 3, Text: "world", Speaker: &speakerA},
			},
			want: map[string]int{"A": 2, "B": 1},
		},
		{
			name: "empty speaker string ignored",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "hello", Speaker: &speakerA},
				{Start: 1, End: 2, Text: "hi", Speaker: &empty},
			},
			want: map[string]int{"A": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupSegmentsBySpeaker(tt.segments)
			for speaker, expectedCount := range tt.want {
				if len(result[speaker]) != expectedCount {
					t.Errorf("speaker %s: got %d segments, want %d", speaker, len(result[speaker]), expectedCount)
				}
			}
			// Check no extra speakers
			if len(result) != len(tt.want) {
				t.Errorf("got %d speakers, want %d", len(result), len(tt.want))
			}
		})
	}
}

func TestSelectBestSegment(t *testing.T) {
	tests := []struct {
		name          string
		segments      []interfaces.TranscriptSegment
		wantNil       bool
		wantDuration  float64 // approximate expected duration
		wantMinDur    float64
		wantMaxDur    float64
	}{
		{
			name:     "empty segments returns nil",
			segments: nil,
			wantNil:  true,
		},
		{
			name: "ideal segment selected",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "short"},            // 1 sec - too short
				{Start: 1, End: 6, Text: "ideal segment"},    // 5 sec - ideal
				{Start: 6, End: 20, Text: "too long segment"}, // 14 sec - too long
			},
			wantMinDur: 4.5,
			wantMaxDur: 5.5,
		},
		{
			name: "long segment trimmed to max",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 15, Text: "very long segment"}, // 15 sec
			},
			wantMinDur: 9.5,
			wantMaxDur: 10.5, // Should be trimmed to MaxSampleDurationSec
		},
		{
			name: "short segments concatenated",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 0.5, Text: "a"},
				{Start: 0.6, End: 1.1, Text: "b"},
				{Start: 1.2, End: 1.7, Text: "c"},
				{Start: 1.8, End: 2.3, Text: "d"},
				{Start: 2.4, End: 2.9, Text: "e"},
			},
			wantMinDur: 2.0, // Should concatenate to meet minimum
			wantMaxDur: 3.0,
		},
		{
			name: "too short returns nil",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 0.5, Text: "a"}, // 0.5 sec with big gaps
				{Start: 10, End: 10.5, Text: "b"},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestSegment(tt.segments)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got segment with duration %.2f", result.End-result.Start)
				}
				return
			}
			if result == nil {
				t.Fatal("expected segment, got nil")
			}
			duration := result.End - result.Start
			if duration < tt.wantMinDur || duration > tt.wantMaxDur {
				t.Errorf("duration %.2f not in range [%.2f, %.2f]", duration, tt.wantMinDur, tt.wantMaxDur)
			}
		})
	}
}

func TestConcatenateSegments(t *testing.T) {
	speaker := "A"
	tests := []struct {
		name       string
		segments   []interfaces.TranscriptSegment
		wantNil    bool
		wantMinDur float64
	}{
		{
			name:     "empty returns nil",
			segments: nil,
			wantNil:  true,
		},
		{
			name: "consecutive segments merged",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 1, Text: "a", Speaker: &speaker},
				{Start: 1.2, End: 2, Text: "b", Speaker: &speaker},
				{Start: 2.1, End: 3, Text: "c", Speaker: &speaker},
			},
			wantMinDur: MinSampleDurationSec,
		},
		{
			name: "gap resets concatenation",
			segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 0.5, Text: "a", Speaker: &speaker},
				{Start: 5, End: 5.5, Text: "b", Speaker: &speaker}, // 4.5 sec gap
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := concatenateSegments(tt.segments)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got segment")
				}
				return
			}
			if result == nil {
				t.Fatal("expected segment, got nil")
			}
			duration := result.End - result.Start
			if duration < tt.wantMinDur {
				t.Errorf("duration %.2f < minimum %.2f", duration, tt.wantMinDur)
			}
		})
	}
}

func TestToSpeakerReferences(t *testing.T) {
	samples := []SpeakerSample{
		{Speaker: "A", Base64Data: "data:audio/mp3;base64,AAAA"},
		{Speaker: "B", Base64Data: "data:audio/mp3;base64,BBBB"},
	}

	refs := ToSpeakerReferences(samples)

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Speaker != "A" || refs[0].ReferenceAudio != "data:audio/mp3;base64,AAAA" {
		t.Errorf("ref[0] mismatch: %+v", refs[0])
	}
	if refs[1].Speaker != "B" || refs[1].ReferenceAudio != "data:audio/mp3;base64,BBBB" {
		t.Errorf("ref[1] mismatch: %+v", refs[1])
	}
}

func TestEncodeAsDataURL(t *testing.T) {
	// Create a temporary file with known content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.mp3")
	content := []byte("test audio content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := encodeAsDataURL(tmpFile)
	if err != nil {
		t.Fatalf("encodeAsDataURL failed: %v", err)
	}

	if !strings.HasPrefix(result, "data:audio/mp3;base64,") {
		t.Errorf("result should start with data URL prefix, got: %s", result[:50])
	}

	// Verify it's valid base64
	encoded := strings.TrimPrefix(result, "data:audio/mp3;base64,")
	if len(encoded) == 0 {
		t.Error("base64 content is empty")
	}
}

func TestExtractSpeakerSamplesNilResult(t *testing.T) {
	samples, err := ExtractSpeakerSamples(context.Background(), nil, "/tmp/test.mp3", "/tmp")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if samples != nil {
		t.Errorf("expected nil samples, got %d", len(samples))
	}
}

func TestExtractSpeakerSamplesEmptySegments(t *testing.T) {
	result := &interfaces.TranscriptResult{
		Segments: []interfaces.TranscriptSegment{},
	}
	samples, err := ExtractSpeakerSamples(context.Background(), result, "/tmp/test.mp3", "/tmp")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if samples != nil {
		t.Errorf("expected nil samples, got %d", len(samples))
	}
}

func TestCleanupSpeakerSamples(t *testing.T) {
	// Create temp files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "speaker_A.mp3")
	file2 := filepath.Join(tmpDir, "speaker_B.mp3")
	os.WriteFile(file1, []byte("test"), 0644)
	os.WriteFile(file2, []byte("test"), 0644)

	samples := []SpeakerSample{
		{Speaker: "A", FilePath: file1},
		{Speaker: "B", FilePath: file2},
	}

	CleanupSpeakerSamples(samples)

	// Verify files are removed
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("file1 should be deleted")
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Errorf("file2 should be deleted")
	}
}
