package postprocessor

import (
	"testing"

	"scriberr/internal/transcription/interfaces"

	"github.com/stretchr/testify/assert"
)

func TestNewAITextPostprocessor_Disabled(t *testing.T) {
	p := NewAITextPostprocessor("", "", false)
	assert.NotNil(t, p)
	assert.False(t, p.enabled)
	assert.Nil(t, p.llmService)
}

func TestNewAITextPostprocessor_NoAPIKey(t *testing.T) {
	p := NewAITextPostprocessor("", "gpt-4o", true)
	assert.NotNil(t, p)
	assert.False(t, p.enabled) // Should be disabled without API key
}

func TestNewAITextPostprocessor_DefaultModel(t *testing.T) {
	p := NewAITextPostprocessor("test-key", "", true)
	assert.NotNil(t, p)
	assert.Equal(t, DefaultModel, p.model)
}

func TestSplitIntoBatches_SmallInput(t *testing.T) {
	p := &AITextPostprocessor{maxSegmentsPerBatch: 50}

	segments := make([]interfaces.TranscriptSegment, 10)
	for i := range segments {
		segments[i] = interfaces.TranscriptSegment{Text: "test"}
	}

	batches := p.splitIntoBatches(segments)
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 10)
}

func TestSplitIntoBatches_LargeInput(t *testing.T) {
	p := &AITextPostprocessor{maxSegmentsPerBatch: 50}

	segments := make([]interfaces.TranscriptSegment, 125)
	for i := range segments {
		segments[i] = interfaces.TranscriptSegment{Text: "test"}
	}

	batches := p.splitIntoBatches(segments)
	assert.Len(t, batches, 3)
	assert.Len(t, batches[0], 50)
	assert.Len(t, batches[1], 50)
	assert.Len(t, batches[2], 25)
}

func TestSplitIntoBatches_ExactBatch(t *testing.T) {
	p := &AITextPostprocessor{maxSegmentsPerBatch: 50}

	segments := make([]interfaces.TranscriptSegment, 100)
	for i := range segments {
		segments[i] = interfaces.TranscriptSegment{Text: "test"}
	}

	batches := p.splitIntoBatches(segments)
	assert.Len(t, batches, 2)
	assert.Len(t, batches[0], 50)
	assert.Len(t, batches[1], 50)
}

func TestParseCleanupResponse_Valid(t *testing.T) {
	response := `[
		{"text": "Hello, world.", "speaker": "A", "start": 0.0, "end": 1.0, "merge_with_next": false},
		{"text": "How are you?", "speaker": "A", "start": 1.0, "end": 2.0, "merge_with_next": false}
	]`

	original := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0},
		{Text: "How are you", Speaker: "A", Start: 1.0, End: 2.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, segments, 2)
	assert.Equal(t, "Hello, world.", segments[0].Text)
	assert.Equal(t, "How are you?", segments[1].Text)
}

func TestParseCleanupResponse_WithMarkdown(t *testing.T) {
	response := "```json\n" + `[
		{"text": "Hello.", "speaker": "A", "start": 0.0, "end": 1.0}
	]` + "\n```"

	original := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, segments, 1)
	assert.Equal(t, "Hello.", segments[0].Text)
}

func TestParseCleanupResponse_PremergedSegments(t *testing.T) {
	// LLM merged 3 segments into 1
	response := `[
		{"text": "你好，我今日去咗買股票。", "speaker": "A", "start": 0.0, "end": 3.0}
	]`

	original := []CleanedSegment{
		{Text: "你好", Speaker: "A", Start: 0.0, End: 1.0},
		{Text: "我今日", Speaker: "A", Start: 1.0, End: 2.0},
		{Text: "去咗買股票", Speaker: "A", Start: 2.0, End: 3.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	// Should return the pre-merged segment
	assert.Len(t, segments, 1)
	assert.Equal(t, "你好，我今日去咗買股票。", segments[0].Text)
	assert.Equal(t, 0.0, segments[0].Start)
	assert.Equal(t, 3.0, segments[0].End)
}

func TestParseCleanupResponse_MoreThanOriginal(t *testing.T) {
	// LLM returned more segments than input - should error
	response := `[
		{"text": "Hello.", "speaker": "A", "start": 0.0, "end": 0.5},
		{"text": "World.", "speaker": "A", "start": 0.5, "end": 1.0}
	]`

	original := []CleanedSegment{
		{Text: "Hello world", Speaker: "A", Start: 0.0, End: 1.0},
	}

	_, err := parseCleanupResponse(response, original)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "segment count increased")
}

func TestParseCleanupResponse_InvalidJSON(t *testing.T) {
	response := `not valid json`

	original := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0},
	}

	_, err := parseCleanupResponse(response, original)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestRebuildFullText(t *testing.T) {
	speaker := "A"
	segments := []interfaces.TranscriptSegment{
		{Text: "Hello,", Speaker: &speaker},
		{Text: "world!", Speaker: &speaker},
		{Text: "[REMOVE]", Speaker: &speaker},
		{Text: "How are you?", Speaker: &speaker},
	}

	text := rebuildFullText(segments)
	assert.Equal(t, "Hello, world! How are you?", text)
}

func TestRebuildFullText_Empty(t *testing.T) {
	text := rebuildFullText(nil)
	assert.Equal(t, "", text)

	text = rebuildFullText([]interfaces.TranscriptSegment{})
	assert.Equal(t, "", text)
}

func TestAppliesTo_Enabled(t *testing.T) {
	p := &AITextPostprocessor{enabled: true}
	assert.True(t, p.AppliesTo(interfaces.ModelCapabilities{}, nil))
}

func TestAppliesTo_Disabled(t *testing.T) {
	p := &AITextPostprocessor{enabled: false}
	assert.False(t, p.AppliesTo(interfaces.ModelCapabilities{}, nil))
}

func TestParseCleanupResponse_PreserveRepeatedStructures(t *testing.T) {
	// Test case: "我想飲牛奶我想飲雞尾酒" should NOT become "我想飲牛奶雞尾酒"
	// The LLM should preserve the repeated "我想" structure
	response := `[
		{"text": "我哋今日去咗邊度，去咗飲酒，但係想飲嗰杯雞尾酒又想飲牛奶。", "speaker": "A", "start": 0.0, "end": 5.0}
	]`

	original := []CleanedSegment{
		{Text: "我哋今日去咗邊度", Speaker: "A", Start: 0.0, End: 1.0},
		{Text: "去咗飲酒", Speaker: "A", Start: 1.0, End: 2.0},
		{Text: "但係想飲嗰杯雞尾酒", Speaker: "A", Start: 2.0, End: 3.5},
		{Text: "又想飲牛奶", Speaker: "A", Start: 3.5, End: 5.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, segments, 1)
	// Verify the repeated structure is maintained (想飲...又想飲...)
	assert.Contains(t, segments[0].Text, "想飲")
	// Should contain both "雞尾酒" and "牛奶"
	assert.Contains(t, segments[0].Text, "雞尾酒")
	assert.Contains(t, segments[0].Text, "牛奶")
}

func TestParseCleanupResponse_RemoveExcessiveRepetition(t *testing.T) {
	// Test case: "是是是" (3+ times) should be reduced to "是是"
	// But "是是" (2 times) should be kept
	response := `[
		{"text": "是是，我明白。", "speaker": "A", "start": 0.0, "end": 2.0}
	]`

	original := []CleanedSegment{
		{Text: "是是是", Speaker: "A", Start: 0.0, End: 1.0},
		{Text: "我明白", Speaker: "A", Start: 1.0, End: 2.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, segments, 1)
	// Should have reduced "是是是" to "是是" (only 2 times)
	assert.Contains(t, segments[0].Text, "是是")
	assert.Contains(t, segments[0].Text, "明白")
}

func TestParseCleanupResponse_KeepNaturalAcknowledgments(t *testing.T) {
	// Test case: Single acknowledgments like "嗯", "是", "明白" should be kept
	response := `[
		{"text": "嗯，我明白了。", "speaker": "A", "start": 0.0, "end": 2.0}
	]`

	original := []CleanedSegment{
		{Text: "嗯", Speaker: "A", Start: 0.0, End: 0.5},
		{Text: "我明白了", Speaker: "A", Start: 0.5, End: 2.0},
	}

	segments, err := parseCleanupResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, segments, 1)
	// Should preserve the "嗯" acknowledgment
	assert.Contains(t, segments[0].Text, "嗯")
	assert.Contains(t, segments[0].Text, "明白")
}
