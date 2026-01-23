package postprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyMerges_EmptyInput(t *testing.T) {
	result := ApplyMerges(nil)
	assert.Nil(t, result)

	result = ApplyMerges([]CleanedSegment{})
	assert.Nil(t, result)
}

func TestApplyMerges_NoMerges(t *testing.T) {
	segments := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0, MergeWithNext: false},
		{Text: "World", Speaker: "A", Start: 1.0, End: 2.0, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 2)
	assert.Equal(t, "Hello", result[0].Text)
	assert.Equal(t, "World", result[1].Text)
}

func TestApplyMerges_SimpleMerge(t *testing.T) {
	segments := []CleanedSegment{
		{Text: "我", Speaker: "A", Start: 0.0, End: 0.5, MergeWithNext: true},
		{Text: "今日", Speaker: "A", Start: 0.5, End: 1.0, MergeWithNext: true},
		{Text: "去咗", Speaker: "A", Start: 1.0, End: 1.5, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 1)
	assert.Equal(t, "我今日去咗", result[0].Text)
	assert.Equal(t, 0.0, result[0].Start)
	assert.Equal(t, 1.5, result[0].End)
	assert.Equal(t, "A", *result[0].Speaker)
}

func TestApplyMerges_MultipleGroups(t *testing.T) {
	segments := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0, MergeWithNext: true},
		{Text: "World", Speaker: "A", Start: 1.0, End: 2.0, MergeWithNext: false},
		{Text: "Good", Speaker: "B", Start: 3.0, End: 4.0, MergeWithNext: true},
		{Text: "Morning", Speaker: "B", Start: 4.0, End: 5.0, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 2)
	assert.Equal(t, "HelloWorld", result[0].Text)
	assert.Equal(t, 0.0, result[0].Start)
	assert.Equal(t, 2.0, result[0].End)
	assert.Equal(t, "GoodMorning", result[1].Text)
	assert.Equal(t, 3.0, result[1].Start)
	assert.Equal(t, 5.0, result[1].End)
}

func TestApplyMerges_RemoveSegments(t *testing.T) {
	segments := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0, MergeWithNext: false},
		{Text: "[REMOVE]", Speaker: "A", Start: 1.0, End: 2.0, MergeWithNext: false},
		{Text: "World", Speaker: "A", Start: 2.0, End: 3.0, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 2)
	assert.Equal(t, "Hello", result[0].Text)
	assert.Equal(t, "World", result[1].Text)
}

func TestApplyMerges_MergeWithRemove(t *testing.T) {
	segments := []CleanedSegment{
		{Text: "Hello", Speaker: "A", Start: 0.0, End: 1.0, MergeWithNext: true},
		{Text: "[REMOVE]", Speaker: "A", Start: 1.0, End: 2.0, MergeWithNext: true},
		{Text: "World", Speaker: "A", Start: 2.0, End: 3.0, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 1)
	assert.Equal(t, "HelloWorld", result[0].Text)
	assert.Equal(t, 0.0, result[0].Start)
	assert.Equal(t, 3.0, result[0].End)
}

func TestApplyMerges_ChainMerge(t *testing.T) {
	// Test case from PRP: consecutive single-word segments
	segments := []CleanedSegment{
		{Text: "我", Speaker: "A", Start: 0.0, End: 0.2, MergeWithNext: true},
		{Text: "今", Speaker: "A", Start: 0.2, End: 0.4, MergeWithNext: true},
		{Text: "日", Speaker: "A", Start: 0.4, End: 0.6, MergeWithNext: true},
		{Text: "好", Speaker: "A", Start: 0.6, End: 0.8, MergeWithNext: true},
		{Text: "開心。", Speaker: "A", Start: 0.8, End: 1.5, MergeWithNext: false},
	}

	result := ApplyMerges(segments)
	assert.Len(t, result, 1)
	assert.Equal(t, "我今日好開心。", result[0].Text)
	assert.Equal(t, 0.0, result[0].Start)
	assert.Equal(t, 1.5, result[0].End)
}

func TestConcatenateTexts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"Hello"}, "Hello"},
		{"multiple", []string{"Hello", "World"}, "HelloWorld"},
		{"chinese", []string{"我", "今日", "好開心"}, "我今日好開心"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := concatenateTexts(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
