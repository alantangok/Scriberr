package postprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"scriberr/internal/llm"
	"scriberr/internal/transcription/interfaces"
	"scriberr/pkg/logger"
)

const (
	DefaultModel            = "gpt-4o"
	DefaultMaxSegmentsPerBatch = 50
)

// AITextPostprocessor uses LLM to clean up transcription results
type AITextPostprocessor struct {
	llmService          *llm.OpenAIService
	model               string
	maxSegmentsPerBatch int
	enabled             bool
}

// NewAITextPostprocessor creates a new AI text postprocessor
func NewAITextPostprocessor(apiKey string, model string, enabled bool) *AITextPostprocessor {
	if model == "" {
		model = DefaultModel
	}

	var llmService *llm.OpenAIService
	if enabled && apiKey != "" {
		llmService = llm.NewOpenAIService(apiKey, nil)
	}

	return &AITextPostprocessor{
		llmService:          llmService,
		model:               model,
		maxSegmentsPerBatch: DefaultMaxSegmentsPerBatch,
		enabled:             enabled && apiKey != "",
	}
}

// ProcessTranscript processes transcription results using LLM
func (p *AITextPostprocessor) ProcessTranscript(
	ctx context.Context,
	result *interfaces.TranscriptResult,
	params map[string]interface{},
) (*interfaces.TranscriptResult, error) {
	if !p.enabled || p.llmService == nil {
		logger.Debug("AI post-processing disabled, returning original result")
		return result, nil
	}

	if len(result.Segments) == 0 {
		return result, nil
	}

	logger.Info("Starting AI post-processing", "segments", len(result.Segments))

	// Process segments in batches
	batches := p.splitIntoBatches(result.Segments)
	allCleaned := make([]CleanedSegment, 0, len(result.Segments))

	for i, batch := range batches {
		logger.Debug("Processing batch", "batch", i+1, "total", len(batches), "segments", len(batch))

		cleaned, err := p.processBatch(ctx, batch)
		if err != nil {
			logger.Warn("Batch processing failed, using original segments",
				"batch", i+1, "error", err)
			// Fallback: convert original segments to cleaned format
			for _, seg := range batch {
				speaker := ""
				if seg.Speaker != nil {
					speaker = *seg.Speaker
				}
				allCleaned = append(allCleaned, CleanedSegment{
					Text:    seg.Text,
					Speaker: speaker,
					Start:   seg.Start,
					End:     seg.End,
				})
			}
			continue
		}

		allCleaned = append(allCleaned, cleaned...)
	}

	// Apply merges and create final result
	mergedSegments := ApplyMerges(allCleaned)

	// Create new result with processed segments
	processedResult := &interfaces.TranscriptResult{
		Text:           rebuildFullText(mergedSegments),
		Language:       result.Language,
		Segments:       mergedSegments,
		WordSegments:   MergeWordSegments(result.WordSegments, result.Segments, mergedSegments),
		Confidence:     result.Confidence,
		ProcessingTime: result.ProcessingTime,
		ModelUsed:      result.ModelUsed,
		Metadata:       result.Metadata,
	}

	if processedResult.Metadata == nil {
		processedResult.Metadata = make(map[string]string)
	}
	processedResult.Metadata["ai_postprocessed"] = "true"
	processedResult.Metadata["postprocessor_model"] = p.model

	logger.Info("AI post-processing complete",
		"original_segments", len(result.Segments),
		"processed_segments", len(mergedSegments))

	return processedResult, nil
}

// ProcessDiarization is a no-op for AI postprocessor (only handles transcript text)
func (p *AITextPostprocessor) ProcessDiarization(
	ctx context.Context,
	result *interfaces.DiarizationResult,
	params map[string]interface{},
) (*interfaces.DiarizationResult, error) {
	return result, nil
}

// AppliesTo determines if this postprocessor should be used
func (p *AITextPostprocessor) AppliesTo(
	capabilities interfaces.ModelCapabilities,
	params map[string]interface{},
) bool {
	return p.enabled
}

// splitIntoBatches splits segments into batches for processing
func (p *AITextPostprocessor) splitIntoBatches(segments []interfaces.TranscriptSegment) [][]interfaces.TranscriptSegment {
	if len(segments) <= p.maxSegmentsPerBatch {
		return [][]interfaces.TranscriptSegment{segments}
	}

	var batches [][]interfaces.TranscriptSegment
	for i := 0; i < len(segments); i += p.maxSegmentsPerBatch {
		end := i + p.maxSegmentsPerBatch
		if end > len(segments) {
			end = len(segments)
		}
		batches = append(batches, segments[i:end])
	}

	return batches
}

// processBatch processes a single batch of segments through LLM
func (p *AITextPostprocessor) processBatch(
	ctx context.Context,
	segments []interfaces.TranscriptSegment,
) ([]CleanedSegment, error) {
	// Convert to input format
	inputSegments := make([]CleanedSegment, len(segments))
	for i, seg := range segments {
		speaker := ""
		if seg.Speaker != nil {
			speaker = *seg.Speaker
		}
		inputSegments[i] = CleanedSegment{
			Text:    seg.Text,
			Speaker: speaker,
			Start:   seg.Start,
			End:     seg.End,
		}
	}

	// Build prompt
	inputJSON, err := json.Marshal(inputSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal segments: %w", err)
	}

	userPrompt := fmt.Sprintf(UserPromptTemplate, string(inputJSON))

	// Call LLM
	messages := []llm.ChatMessage{
		{Role: "system", Content: SystemPromptCleanup},
		{Role: "user", Content: userPrompt},
	}

	resp, err := p.llmService.ChatCompletion(ctx, p.model, messages, 0)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	// Parse response
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	cleaned, err := parseCleanupResponse(content, inputSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return cleaned, nil
}

// parseCleanupResponse parses the LLM response JSON
// Accepts pre-merged segments from LLM (fewer segments than input is OK)
func parseCleanupResponse(content string, originalSegments []CleanedSegment) ([]CleanedSegment, error) {
	// Strip markdown code blocks if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var segments []CleanedSegment
	if err := json.Unmarshal([]byte(content), &segments); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// LLM returned more segments than input - that's an error
	if len(segments) > len(originalSegments) {
		return nil, fmt.Errorf("segment count increased: expected <= %d, got %d", len(originalSegments), len(segments))
	}

	// If counts match exactly, use as-is (ideal case with merge_with_next flags)
	if len(segments) == len(originalSegments) {
		return segments, nil
	}

	// LLM pre-merged segments - map them back using timestamps
	logger.Debug("LLM pre-merged segments, mapping back",
		"original", len(originalSegments), "received", len(segments))

	return mapPremergedSegments(segments, originalSegments), nil
}

// mapPremergedSegments maps LLM pre-merged segments back using timestamp overlap
// The LLM returns merged segments - we need to match them to originals by time overlap
// IMPORTANT: This preserves ALL LLM-returned content while maintaining time alignment
func mapPremergedSegments(llmSegments, originalSegments []CleanedSegment) []CleanedSegment {
	if len(llmSegments) == 0 {
		return originalSegments
	}

	// Simple approach: use the LLM segments directly since they contain the cleaned content
	// The LLM has already done the merging work, just use its output
	// We don't need to map back to original boundaries - the LLM output is what we want
	result := make([]CleanedSegment, len(llmSegments))
	copy(result, llmSegments)

	logger.Debug("Using LLM pre-merged segments directly",
		"original_count", len(originalSegments),
		"llm_count", len(llmSegments))

	return result
}

// rebuildFullText reconstructs the full text from segments
func rebuildFullText(segments []interfaces.TranscriptSegment) string {
	var parts []string
	for _, seg := range segments {
		if seg.Text != "" && seg.Text != "[REMOVE]" {
			parts = append(parts, seg.Text)
		}
	}
	return strings.Join(parts, " ")
}
