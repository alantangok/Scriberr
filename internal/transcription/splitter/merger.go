package splitter

import (
	"fmt"
	"strings"
	"time"

	"scriberr/internal/transcription/interfaces"
)

// MergeResults combines transcript results from multiple chunks
func MergeResults(results []*interfaces.TranscriptResult, chunks []ChunkInfo) *interfaces.TranscriptResult {
	if len(results) == 0 {
		return nil
	}

	if len(results) == 1 {
		return results[0]
	}

	merged := &interfaces.TranscriptResult{
		Segments:     make([]interfaces.TranscriptSegment, 0),
		WordSegments: make([]interfaces.TranscriptWord, 0),
		Metadata:     make(map[string]string),
	}

	var textParts []string
	var totalProcessingTime time.Duration
	var totalConfidence float64

	for i, result := range results {
		if result == nil {
			continue
		}

		// Get time offset for this chunk
		var timeOffset float64
		if i < len(chunks) {
			timeOffset = chunks[i].StartTime
		}

		// Append text
		if result.Text != "" {
			textParts = append(textParts, strings.TrimSpace(result.Text))
		}

		// Adjust and append segments
		for _, seg := range result.Segments {
			adjustedSeg := interfaces.TranscriptSegment{
				Start:    seg.Start + timeOffset,
				End:      seg.End + timeOffset,
				Text:     seg.Text,
				Speaker:  seg.Speaker,
				Language: seg.Language,
			}
			merged.Segments = append(merged.Segments, adjustedSeg)
		}

		// Adjust and append word segments
		for _, word := range result.WordSegments {
			adjustedWord := interfaces.TranscriptWord{
				Start:   word.Start + timeOffset,
				End:     word.End + timeOffset,
				Word:    word.Word,
				Score:   word.Score,
				Speaker: word.Speaker,
			}
			merged.WordSegments = append(merged.WordSegments, adjustedWord)
		}

		// Accumulate processing time and confidence
		totalProcessingTime += result.ProcessingTime
		totalConfidence += result.Confidence

		// Use first result's language and model
		if merged.Language == "" && result.Language != "" {
			merged.Language = result.Language
		}
		if merged.ModelUsed == "" && result.ModelUsed != "" {
			merged.ModelUsed = result.ModelUsed
		}

		// Merge metadata
		for k, v := range result.Metadata {
			merged.Metadata[k] = v
		}
	}

	// Combine text
	merged.Text = strings.Join(textParts, " ")

	// Average confidence
	if len(results) > 0 {
		merged.Confidence = totalConfidence / float64(len(results))
	}

	merged.ProcessingTime = totalProcessingTime
	merged.Metadata["chunks_processed"] = fmt.Sprintf("%d", len(results))

	return merged
}
