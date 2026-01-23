# PL002: AI-Powered Transcript Post-Processing

**Date**: 2026-01-23
**Status**: Implementation Complete
**Priority**: High

## Problem Statement

Raw transcription output from gpt-4o-transcribe-diarize has several quality issues:

1. **Missing punctuation/symbols**: No proper commas, periods, question marks in Cantonese/Chinese transcripts
2. **Contextual errors**: Nonsensical phrases that don't fit context (e.g., stock discussion suddenly mentions "drink milk" - clearly a transcription error)
3. **Excessive repetition**: Filler words, repeated phrases, meaningless single-word segments
4. **Fragmented segments**: Multiple consecutive 1-word segments that should be merged

**User requirement summary** (Cantonese):
- Add punctuation and make text more readable
- Use context awareness to fix errors (if discussing stocks, random unrelated sentence is likely wrong)
- Remove repetitive/meaningless words
- Merge consecutive single-word segments into proper segments (preserving start/end timestamps)

## Solution

Create an AI-powered post-processor using GPT-4o via existing LLM service to:
1. **Clean & punctuate**: Add proper punctuation, symbols, formatting
2. **Context-aware correction**: Fix transcription errors based on surrounding context
3. **Deduplication**: Remove excessive repetition while preserving meaning
4. **Segment merging**: Combine fragmented segments intelligently

## Architecture Decision

**Key Insight**: Leverage existing infrastructure

- **Existing LLM Service**: `internal/llm/openai.go` already has ChatCompletion working
- **Existing Postprocessor Interface**: `internal/transcription/interfaces/interfaces.go` defines `Postprocessor` interface
- **Existing Pipeline**: `internal/transcription/pipeline/pipeline.go` has `RegisterPostprocessor` and `TextPostprocessor` placeholder

**Approach**: Create `AITextPostprocessor` that implements `Postprocessor` interface and uses existing LLM service.

## Data Flow

```
Raw Transcript (from transcription)
       |
       v
[AITextPostprocessor.ProcessTranscript]
       |
       +---> Chunk segments into batches (to fit context window)
       |
       v
[LLM ChatCompletion] x N batches
       |
       +---> System prompt: transcript cleanup rules
       +---> User prompt: JSON segments batch
       +---> Response: cleaned JSON segments
       |
       v
[Parse & Validate]
       |
       +---> Validate JSON response
       +---> Preserve original timestamps
       +---> Handle merge requests
       |
       v
Cleaned Transcript (same structure, improved text)
```

## Implementation Plan

### Phase 1: AI Postprocessor Core

**File**: `internal/transcription/postprocessor/ai_postprocessor.go` (NEW)

1. **AITextPostprocessor struct**
   ```
   - llmService: *llm.OpenAIService
   - model: string (default: gpt-4o)
   - maxSegmentsPerBatch: int (default: 50)
   - enabled: bool (from config/env)
   ```

2. **ProcessTranscript method**
   - Split segments into batches
   - For each batch, call LLM with cleanup prompt
   - Parse response and update segments
   - Handle segment merging (adjust timestamps)
   - Return cleaned result

3. **buildCleanupPrompt function**
   - System prompt with rules:
     - Add punctuation (commas, periods, question marks)
     - Fix contextual errors based on surrounding text
     - Remove excessive repetition
     - Mark segments for merging (consecutive 1-word segments)
   - User prompt: JSON array of segments

4. **parseCleanupResponse function**
   - Parse LLM JSON response
   - Validate structure matches expected format
   - Handle merge markers
   - Preserve original timestamps

### Phase 2: Segment Merging Logic

**File**: `internal/transcription/postprocessor/segment_merger.go` (NEW)

1. **MergeRequest struct**
   ```
   - StartIndex: int
   - EndIndex: int
   - MergedText: string
   ```

2. **ApplyMerges function**
   - Input: original segments, merge requests
   - Output: new segment list with merges applied
   - Logic:
     - For each merge: new segment has start=first.start, end=last.end
     - Remove merged segments, insert single merged segment
     - Adjust word_segments if present

### Phase 3: Integration

**File**: `internal/transcription/unified_service.go` (UPDATE)

1. **Add postprocessor field**
   ```
   postprocessor *postprocessor.AITextPostprocessor
   ```

2. **Enable post-processing after transcription**
   - After transcription completes, before saving
   - Call postprocessor.ProcessTranscript
   - Save processed result

**File**: `internal/config/config.go` (UPDATE)

1. **Add config options**
   ```
   EnableAIPostProcessing bool
   PostProcessingModel string
   ```

### Phase 4: Prompt Engineering

**System Prompt** (stored as constant):
```
You are a transcript post-processor for Cantonese/Chinese audio. Your task is to clean up raw transcription output.

Rules:
1. ADD PUNCTUATION: Add commas, periods, question marks where appropriate
2. FIX CONTEXT ERRORS: If a phrase makes no sense given surrounding context, either:
   - Correct it to what was likely said
   - Mark as [REMOVE] if unrecoverable
3. REMOVE REPETITION: Remove excessive filler words and repeated phrases
4. MERGE FRAGMENTS: For consecutive 1-word segments, suggest merging by setting merge_with_next=true

Input format: JSON array of segments with {text, speaker, start, end}
Output format: Same JSON array with cleaned text and optional merge_with_next flag

IMPORTANT:
- Preserve speaker labels exactly
- Do NOT modify start/end timestamps
- Keep the same number of segments unless merging
- Return valid JSON only
```

## File Changes Summary

| File | Type | Description |
|------|------|-------------|
| `internal/transcription/postprocessor/ai_postprocessor.go` | NEW | Main AI postprocessor implementation |
| `internal/transcription/postprocessor/segment_merger.go` | NEW | Segment merging logic |
| `internal/transcription/postprocessor/prompts.go` | NEW | LLM prompt templates |
| `internal/transcription/unified_service.go` | UPDATE | Integration with postprocessor |
| `internal/config/config.go` | UPDATE | Add config options |

## Implementation Tasks

### Phase 1: Core Postprocessor (Foundation)
- [x] Create `internal/transcription/postprocessor/` directory
- [x] Implement `AITextPostprocessor` struct with LLM service integration
- [x] Implement `ProcessTranscript` method with batching logic
- [x] Implement `buildCleanupPrompt` with rules
- [x] Implement `parseCleanupResponse` with JSON validation

### Phase 2: Segment Merging
- [x] Implement `MergeRequest` struct
- [x] Implement `ApplyMerges` function with timestamp handling
- [x] Handle word_segments merging

### Phase 3: Integration & Config
- [x] Add `EnableAIPostProcessing` to config
- [x] Add `PostProcessingModel` to config (default: gpt-4o)
- [x] Update `UnifiedTranscriptionService` to use postprocessor
- [x] Add postprocessor initialization in service constructor

### Phase 4: Testing
- [x] Unit tests for segment merging
- [x] Unit tests for prompt building
- [x] Unit tests for response parsing
- [ ] Integration test with mock LLM response
- [ ] E2E test with real OpenAI API

## API Response Format

**Input to LLM (batch of segments)**:
```json
[
  {"text": "我今日", "speaker": "A", "start": 0.0, "end": 0.5},
  {"text": "去", "speaker": "A", "start": 0.5, "end": 0.7},
  {"text": "咗", "speaker": "A", "start": 0.7, "end": 0.9},
  {"text": "買", "speaker": "A", "start": 0.9, "end": 1.1},
  {"text": "股票", "speaker": "A", "start": 1.1, "end": 1.5},
  {"text": "飲奶味", "speaker": "A", "start": 1.5, "end": 2.0},
  {"text": "升咗好多", "speaker": "A", "start": 2.0, "end": 2.8}
]
```

**Output from LLM**:
```json
[
  {"text": "我今日", "speaker": "A", "start": 0.0, "end": 0.5, "merge_with_next": true},
  {"text": "去", "speaker": "A", "start": 0.5, "end": 0.7, "merge_with_next": true},
  {"text": "咗", "speaker": "A", "start": 0.7, "end": 0.9, "merge_with_next": true},
  {"text": "買", "speaker": "A", "start": 0.9, "end": 1.1, "merge_with_next": true},
  {"text": "股票，", "speaker": "A", "start": 1.1, "end": 1.5, "merge_with_next": false},
  {"text": "[REMOVE]", "speaker": "A", "start": 1.5, "end": 2.0, "merge_with_next": false},
  {"text": "升咗好多。", "speaker": "A", "start": 2.0, "end": 2.8, "merge_with_next": false}
]
```

**Final merged result**:
```json
[
  {"text": "我今日去咗買股票，", "speaker": "A", "start": 0.0, "end": 1.5},
  {"text": "升咗好多。", "speaker": "A", "start": 2.0, "end": 2.8}
]
```

## Configuration

**Environment variables**:
```bash
# Enable AI post-processing (default: false)
ENABLE_AI_POST_PROCESSING=true

# Model to use for post-processing (default: gpt-4o)
POST_PROCESSING_MODEL=gpt-4o

# Max segments per LLM batch (default: 50)
POST_PROCESSING_BATCH_SIZE=50
```

## Success Criteria

- Punctuation added to >90% of sentences
- Contextual errors reduced by >70% (measured via sample review)
- Fragmented segments merged appropriately
- Processing time adds <30 seconds per minute of audio
- No increase in transcription failures

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM returns invalid JSON | High | Strict validation, fallback to original |
| LLM over-corrects text | Medium | Conservative prompt, preserve uncertain text |
| API cost increase | Medium | Batch efficiently, make optional via config |
| Processing time increase | Low | Async processing, batch optimization |

## Cost Estimate

- GPT-4o: ~$5/1M input tokens, ~$15/1M output tokens
- Typical 10-minute transcript: ~2000 tokens input, ~2000 tokens output
- Cost per transcript: ~$0.04 USD
- Monthly estimate (100 transcripts): ~$4 USD

## Dependencies

- Existing `internal/llm/openai.go` - no new dependencies
- Same OpenAI API key used for transcription

## Future Enhancements (Out of Scope)

- Custom vocabulary/terms list for domain-specific transcription
- Speaker name assignment (map A/B to actual names)
- Multi-language support beyond Cantonese/Chinese
- Confidence scoring for corrections made

## Implementation Complete

**Date**: 2026-01-23
**Files Created**:
- `internal/transcription/postprocessor/ai_postprocessor.go` - Main AI postprocessor implementation
- `internal/transcription/postprocessor/segment_merger.go` - Segment merging logic
- `internal/transcription/postprocessor/prompts.go` - LLM prompt templates
- `internal/transcription/postprocessor/ai_postprocessor_test.go` - Unit tests for postprocessor
- `internal/transcription/postprocessor/segment_merger_test.go` - Unit tests for merger

**Files Modified**:
- `internal/config/config.go` - Added AI post-processing config options
- `internal/transcription/unified_service.go` - Integrated postprocessor into pipeline
- `internal/transcription/queue_integration.go` - Added SetAIPostprocessor method
- `cmd/server/main.go` - Configure postprocessor on startup

**Clean Code Compliance**: ✅ All files pass review
**Unit Tests**: 22 tests passing
