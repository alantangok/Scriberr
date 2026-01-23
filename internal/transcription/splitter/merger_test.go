package splitter

import (
	"testing"

	"scriberr/internal/transcription/interfaces"
)

func TestMergeResultsEmpty(t *testing.T) {
	result := MergeResults(nil, nil, false)
	if result != nil {
		t.Error("expected nil for empty results")
	}
}

func TestMergeResultsSingle(t *testing.T) {
	input := &interfaces.TranscriptResult{
		Text:     "hello world",
		Language: "en",
	}
	result := MergeResults([]*interfaces.TranscriptResult{input}, nil, false)
	if result != input {
		t.Error("single result should return same pointer")
	}
}

func TestMergeResultsMultipleWithoutSpeakerRefs(t *testing.T) {
	speakerA := "A"
	speakerB := "B"

	results := []*interfaces.TranscriptResult{
		{
			Text:     "chunk one",
			Language: "en",
			Segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 5, Text: "hello", Speaker: &speakerA},
				{Start: 5, End: 10, Text: "world", Speaker: &speakerB},
			},
		},
		{
			Text:     "chunk two",
			Language: "en",
			Segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 5, Text: "foo", Speaker: &speakerA},
				{Start: 5, End: 10, Text: "bar", Speaker: &speakerB},
			},
		},
	}

	chunks := []ChunkInfo{
		{StartTime: 0},
		{StartTime: 60}, // Second chunk starts at 60 seconds
	}

	merged := MergeResults(results, chunks, false)

	if merged == nil {
		t.Fatal("merged result is nil")
	}

	// Check text combined
	if merged.Text != "chunk one chunk two" {
		t.Errorf("text mismatch: %s", merged.Text)
	}

	// Check segments count
	if len(merged.Segments) != 4 {
		t.Errorf("expected 4 segments, got %d", len(merged.Segments))
	}

	// Check speaker prefixes (without speaker refs, should have chunk prefix)
	// First chunk: 0-A, 0-B
	// Second chunk: 1-A, 1-B
	if *merged.Segments[0].Speaker != "0-A" {
		t.Errorf("segment[0] speaker should be 0-A, got %s", *merged.Segments[0].Speaker)
	}
	if *merged.Segments[1].Speaker != "0-B" {
		t.Errorf("segment[1] speaker should be 0-B, got %s", *merged.Segments[1].Speaker)
	}
	if *merged.Segments[2].Speaker != "1-A" {
		t.Errorf("segment[2] speaker should be 1-A, got %s", *merged.Segments[2].Speaker)
	}
	if *merged.Segments[3].Speaker != "1-B" {
		t.Errorf("segment[3] speaker should be 1-B, got %s", *merged.Segments[3].Speaker)
	}

	// Check time offsets
	if merged.Segments[2].Start != 60 {
		t.Errorf("segment[2] start should be 60, got %.2f", merged.Segments[2].Start)
	}
	if merged.Segments[3].End != 70 {
		t.Errorf("segment[3] end should be 70, got %.2f", merged.Segments[3].End)
	}

	// Check metadata
	if merged.Metadata["speaker_references_used"] != "" {
		t.Error("speaker_references_used should not be set")
	}
}

func TestMergeResultsMultipleWithSpeakerRefs(t *testing.T) {
	speakerA := "A"
	speakerB := "B"

	results := []*interfaces.TranscriptResult{
		{
			Text:     "chunk one",
			Language: "en",
			Segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 5, Text: "hello", Speaker: &speakerA},
				{Start: 5, End: 10, Text: "world", Speaker: &speakerB},
			},
		},
		{
			Text:     "chunk two",
			Language: "en",
			Segments: []interfaces.TranscriptSegment{
				{Start: 0, End: 5, Text: "foo", Speaker: &speakerA},
				{Start: 5, End: 10, Text: "bar", Speaker: &speakerB},
			},
		},
	}

	chunks := []ChunkInfo{
		{StartTime: 0},
		{StartTime: 60},
	}

	// With speaker references, speakers should NOT be prefixed
	merged := MergeResults(results, chunks, true)

	if merged == nil {
		t.Fatal("merged result is nil")
	}

	// Check speakers NOT prefixed when speaker refs used
	if *merged.Segments[0].Speaker != "A" {
		t.Errorf("segment[0] speaker should be A, got %s", *merged.Segments[0].Speaker)
	}
	if *merged.Segments[1].Speaker != "B" {
		t.Errorf("segment[1] speaker should be B, got %s", *merged.Segments[1].Speaker)
	}
	if *merged.Segments[2].Speaker != "A" {
		t.Errorf("segment[2] speaker should be A, got %s", *merged.Segments[2].Speaker)
	}
	if *merged.Segments[3].Speaker != "B" {
		t.Errorf("segment[3] speaker should be B, got %s", *merged.Segments[3].Speaker)
	}

	// Check metadata
	if merged.Metadata["speaker_references_used"] != "true" {
		t.Error("speaker_references_used should be 'true'")
	}
}

func TestAdjustSpeakerLabel(t *testing.T) {
	speakerA := "A"
	speakerWithPrefix := "Speaker A"
	empty := ""

	tests := []struct {
		name            string
		speaker         *string
		chunkIndex      int
		totalChunks     int
		speakerRefsUsed bool
		want            string
		wantNil         bool
	}{
		{
			name:    "nil speaker stays nil",
			speaker: nil,
			wantNil: true,
		},
		{
			name:    "empty speaker stays empty",
			speaker: &empty,
			want:    "",
		},
		{
			name:            "single chunk no prefix",
			speaker:         &speakerA,
			chunkIndex:      0,
			totalChunks:     1,
			speakerRefsUsed: false,
			want:            "A",
		},
		{
			name:            "multi chunk without refs gets prefix",
			speaker:         &speakerA,
			chunkIndex:      1,
			totalChunks:     3,
			speakerRefsUsed: false,
			want:            "1-A",
		},
		{
			name:            "multi chunk with refs no prefix",
			speaker:         &speakerA,
			chunkIndex:      1,
			totalChunks:     3,
			speakerRefsUsed: true,
			want:            "A",
		},
		{
			name:            "speaker prefix trimmed",
			speaker:         &speakerWithPrefix,
			chunkIndex:      0,
			totalChunks:     2,
			speakerRefsUsed: false,
			want:            "0-A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustSpeakerLabel(tt.speaker, tt.chunkIndex, tt.totalChunks, tt.speakerRefsUsed)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %s", *result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if *result != tt.want {
				t.Errorf("got %s, want %s", *result, tt.want)
			}
		})
	}
}
