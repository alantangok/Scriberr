package postprocessor

import (
	"scriberr/internal/transcription/interfaces"
)

// CleanedSegment represents an LLM-processed segment with merge flag
type CleanedSegment struct {
	Text          string  `json:"text"`
	Speaker       string  `json:"speaker"`
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	MergeWithNext bool    `json:"merge_with_next,omitempty"`
}

// ApplyMerges processes cleaned segments and applies merge operations
// Returns the final merged segments preserving original timestamps
func ApplyMerges(cleaned []CleanedSegment) []interfaces.TranscriptSegment {
	if len(cleaned) == 0 {
		return nil
	}

	result := make([]interfaces.TranscriptSegment, 0, len(cleaned))
	i := 0

	for i < len(cleaned) {
		seg := cleaned[i]

		// Skip segments marked for removal
		if seg.Text == "[REMOVE]" {
			i++
			continue
		}

		// Find merge chain
		mergeEnd := i
		for mergeEnd < len(cleaned)-1 && cleaned[mergeEnd].MergeWithNext {
			mergeEnd++
		}

		if mergeEnd > i {
			// Merge segments from i to mergeEnd
			merged := mergeSegmentRange(cleaned, i, mergeEnd)
			if merged != nil {
				result = append(result, *merged)
			}
			i = mergeEnd + 1
		} else {
			// No merge needed, just convert
			speaker := seg.Speaker
			result = append(result, interfaces.TranscriptSegment{
				Text:    seg.Text,
				Speaker: &speaker,
				Start:   seg.Start,
				End:     seg.End,
			})
			i++
		}
	}

	return result
}

// mergeSegmentRange merges segments from startIdx to endIdx inclusive
func mergeSegmentRange(segments []CleanedSegment, startIdx, endIdx int) *interfaces.TranscriptSegment {
	if startIdx > endIdx || startIdx >= len(segments) {
		return nil
	}

	var texts []string
	for i := startIdx; i <= endIdx && i < len(segments); i++ {
		if segments[i].Text != "[REMOVE]" {
			texts = append(texts, segments[i].Text)
		}
	}

	if len(texts) == 0 {
		return nil
	}

	// Use first segment's start time and last segment's end time
	mergedText := concatenateTexts(texts)
	speaker := segments[startIdx].Speaker

	return &interfaces.TranscriptSegment{
		Text:    mergedText,
		Speaker: &speaker,
		Start:   segments[startIdx].Start,
		End:     segments[endIdx].End,
	}
}

// concatenateTexts joins text segments intelligently
func concatenateTexts(texts []string) string {
	if len(texts) == 0 {
		return ""
	}
	if len(texts) == 1 {
		return texts[0]
	}

	result := texts[0]
	for i := 1; i < len(texts); i++ {
		result += texts[i]
	}
	return result
}

// MergeWordSegments merges word-level segments based on the transcript segment merges
func MergeWordSegments(
	words []interfaces.TranscriptWord,
	originalSegments []interfaces.TranscriptSegment,
	mergedSegments []interfaces.TranscriptSegment,
) []interfaces.TranscriptWord {
	if len(words) == 0 || len(mergedSegments) == 0 {
		return words
	}

	// Create a mapping of time ranges to new speakers
	result := make([]interfaces.TranscriptWord, 0, len(words))
	for _, word := range words {
		// Find which merged segment this word belongs to
		for _, seg := range mergedSegments {
			if word.Start >= seg.Start && word.End <= seg.End {
				word.Speaker = seg.Speaker
				break
			}
		}
		result = append(result, word)
	}

	return result
}
