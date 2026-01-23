# PL001: GPT-4o-Transcribe-Diarize Speaker Reference Implementation

**Date**: 2026-01-23
**Status**: Implemented
**Priority**: High

## Problem Statement

When using `gpt-4o-transcribe-diarize` with chunked audio (required for files >25MB or >5min), speaker labels (A, B, C) do NOT persist across chunks. Each chunk is diarized independently, resulting in:

- Chunk 1: Speaker A = "John", Speaker B = "Jane"
- Chunk 2: Speaker A = "Jane", Speaker B = "John" (swapped!)

**Current workaround**: Prefix speakers with chunk index (e.g., "0-A", "1-B") - functional but not ideal.

## Solution

OpenAI's API supports `known_speaker_references` parameter to provide audio samples of known speakers. By extracting speaker samples from the first chunk, we can pass them to subsequent chunks for consistent identification.

## API Format

```json
{
  "model": "gpt-4o-transcribe-diarize",
  "file": "<audio_chunk>",
  "response_format": "diarized_json",
  "chunking_strategy": "auto",
  "known_speaker_references": [
    {
      "speaker": "A",
      "reference_audio": "data:audio/mp3;base64,<base64_encoded_audio>"
    },
    {
      "speaker": "B",
      "reference_audio": "data:audio/mp3;base64,<base64_encoded_audio>"
    }
  ]
}
```

## Implementation Plan

### Phase 1: Speaker Sample Extraction

**File**: `internal/transcription/splitter/speaker_samples.go` (new)

1. **SpeakerSample struct**
   ```go
   type SpeakerSample struct {
       Speaker      string  // "A", "B", "C"
       StartTime    float64 // Start time in source audio
       EndTime      float64 // End time in source audio
       FilePath     string  // Path to extracted audio clip
       Base64Data   string  // Base64 encoded audio for API
   }
   ```

2. **ExtractSpeakerSamples function**
   - Input: First chunk's transcription result + chunk audio file
   - Logic:
     1. Parse segments to find unique speakers
     2. For each speaker, find a segment with 2-10 seconds of speech
     3. Use ffmpeg to extract audio clip
     4. Encode as base64 data URL
   - Output: []SpeakerSample

3. **FFmpeg extraction command**
   ```bash
   ffmpeg -i chunk_000.mp3 -ss 5.2 -t 4.5 -ar 16000 -ac 1 -c:a libmp3lame -b:a 64k speaker_A.mp3
   ```

### Phase 2: OpenAI Adapter Modification

**File**: `internal/transcription/adapters/openai_adapter.go`

1. **Add SpeakerReference to params**
   ```go
   type SpeakerReference struct {
       Speaker        string `json:"speaker"`
       ReferenceAudio string `json:"reference_audio"`
   }
   ```

2. **Modify Transcribe method**
   - Check for `known_speaker_references` in params
   - If present, add to multipart form:
     ```go
     for i, ref := range speakerRefs {
         writer.WriteField(fmt.Sprintf("known_speaker_references[%d][speaker]", i), ref.Speaker)
         writer.WriteField(fmt.Sprintf("known_speaker_references[%d][reference_audio]", i), ref.ReferenceAudio)
     }
     ```

### Phase 3: Unified Service Integration

**File**: `internal/transcription/unified_service.go`

1. **Modify transcribeWithSplitting**
   - Current flow:
     ```
     for each chunk:
         transcribe(chunk)
     merge(results)
     ```

   - New flow:
     ```
     result0 = transcribe(chunk0)  // No references
     speakerSamples = extractSpeakerSamples(result0, chunk0)

     for chunk1..N:
         params["known_speaker_references"] = speakerSamples
         transcribe(chunk, params)

     merge(results)  // No prefix needed now!
     ```

### Phase 4: Merger Update

**File**: `internal/transcription/splitter/merger.go`

1. **Conditional speaker prefixing**
   - If speaker references were used → speakers are consistent → no prefix
   - If no references (fallback) → keep current prefix logic

2. **Add metadata flag**
   ```go
   merged.Metadata["speaker_references_used"] = "true"
   ```

## File Changes Summary

| File | Changes |
|------|---------|
| `splitter/speaker_samples.go` | NEW - Speaker sample extraction |
| `adapters/openai_adapter.go` | Add known_speaker_references support |
| `unified_service.go` | Two-pass transcription orchestration |
| `splitter/merger.go` | Conditional speaker prefix logic |

## Testing Plan

1. **Unit Tests**
   - `speaker_samples_test.go`: Test extraction with mock segments
   - Test base64 encoding/decoding
   - Test ffmpeg command generation

2. **Integration Tests**
   - 10-minute test audio with 2 speakers
   - Verify speaker A stays as A across chunks
   - Compare with/without references

3. **Edge Cases**
   - Single speaker audio
   - >4 speakers (OpenAI limit unknown)
   - Very short segments (<2 sec)
   - Empty/silent chunks

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| API format changes | High | Validate response, fallback to prefix method |
| Extraction fails | Medium | Use best available segment, log warning |
| Performance overhead | Low | Extraction is fast (<1 sec per speaker) |
| Speaker mismatch still occurs | Medium | Keep prefix as fallback option |

## Dependencies

- ffmpeg (already required for chunking)
- No new Go packages needed

## Rollout

1. **Phase 1**: Implement behind feature flag `ENABLE_SPEAKER_REFERENCES=true`
2. **Phase 2**: Test on production with specific jobs
3. **Phase 3**: Enable by default, keep fallback

## Success Metrics

- Speaker consistency across chunks improves from ~50% to >95%
- No increase in transcription failures
- Minimal processing time increase (<5%)

## Appendix: Sample Extraction Algorithm

```
Input: segments[], minDuration=2s, maxDuration=10s

For each unique speaker:
    Find all segments by this speaker
    Sort by duration (descending)

    For each segment:
        If duration >= minDuration AND duration <= maxDuration:
            Select this segment
            Break

    If no segment found:
        Concatenate consecutive segments until >= minDuration
        Or use longest available segment

    Extract audio using ffmpeg
    Encode as base64 data URL
```

## ✅ Implementation Complete

**Date**: 2026-01-23
**Commits**: bd9fe40, d950c02, 9a72d14, bf891cd

### Files Modified/Created

- `internal/transcription/splitter/speaker_samples.go` (NEW - 256 lines)
  - SpeakerSample and SpeakerReference structs
  - ExtractSpeakerSamples, ToSpeakerReferences functions
  - Segment selection algorithm with concatenation fallback
  - FFmpeg extraction and base64 encoding

- `internal/transcription/adapters/openai_adapter.go` (+25 lines)
  - Added known_speaker_references multipart form fields
  - Included in retry logic for robustness

- `internal/transcription/unified_service.go` (+30 lines)
  - Two-pass transcription in transcribeWithSplitting
  - Speaker sample extraction after first chunk
  - Reference passing to subsequent chunks

- `internal/transcription/splitter/merger.go` (+25 lines)
  - Conditional speaker prefixing via speakerRefsUsed flag
  - adjustSpeakerLabel helper function
  - Metadata flag for speaker_references_used

### Test Coverage

- `internal/transcription/splitter/speaker_samples_test.go` (NEW - 240 lines)
- `internal/transcription/splitter/merger_test.go` (NEW - 210 lines)
- **22 tests, all passing**

### Clean Code Compliance

- ✅ All new files < 300 lines
- ✅ All functions < 30 lines
- ✅ Max 3 indentation levels
- ✅ Single responsibility per function
- ✅ No hardcoded values (constants used)
- ✅ DRY - No duplication
- ✅ Clear naming conventions

### Notes

- Feature is automatically enabled when using gpt-4o-transcribe-diarize with chunked audio
- Falls back to chunk prefix (0-A, 1-B) if sample extraction fails
- No new dependencies required (ffmpeg already in use)
