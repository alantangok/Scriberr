# OpenAI Transcription Implementation Notes

## GPT-4o-Transcribe Models

### Model Variants
- `gpt-4o-transcribe`: Standard transcription
- `gpt-4o-transcribe-diarize`: Transcription with speaker diarization

### Known Limitations (from Azure AI docs)

| Parameter | Value |
|-----------|-------|
| Max Output Tokens | 2,000 |
| Context Window | 16,000 |
| Max Duration | 1,500 sec (25 min) |
| Max File Size | 25 MB |
| Chunk Limit (diarize) | 1,400 sec (~23 min) |

### Diarize-Specific Issues
- **No Realtime API** - synchronous HTTP only
- **No prompting** - can't provide context between chunks
- **Speaker labels don't persist** - each chunk gets independent A, B, C labels
- **Connection timeout** - OpenAI disconnects after ~60 seconds for larger files
- **Truncation at 8-9 minutes** - 2,000 tokens ≈ 1,500 words ≈ 8-10 min of speech

### Chunking Strategy
- **Recommended**: 2-minute chunks (well under 2,000 tokens each)
- **Tested working**: 1-2 minute chunks
- **FAILS**: 5+ minute chunks (connection abort)
- **Required parameter**: `chunking_strategy="auto"` for diarize model

### HTTP/2 Issues
OpenAI API (via Cloudflare) has issues with HTTP/2 on long-running requests.

**Solution**: Force HTTP/1.1 in Go HTTP client:
```go
Transport: &http.Transport{
    ForceAttemptHTTP2: false,
    TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
}
```

### Response Format Differences

Standard whisper-1:
```json
{"segments": [{"id": 0, "text": "...", "start": 0.0, "end": 1.5}]}
```

Diarized gpt-4o:
```json
{"segments": [{"id": "seg_0", "speaker": "A", "text": "...", "start": 0.0, "end": 1.5, "type": "transcript.text.segment"}]}
```

Note: `id` is string in diarized format, int in standard format.

### Cross-Chunk Speaker Identity
- Speaker labels (A, B, C) DO NOT persist across chunks
- Each chunk is diarized independently
- **Workaround**: Extract 2-10 second speaker samples from first chunk as reference (implemented via `known_speaker_references`)

## Key Files
- Adapter: `internal/transcription/adapters/openai_adapter.go`
- Splitter: `internal/transcription/splitter/splitter.go`
- Pipeline: `internal/transcription/pipeline/`

## Testing

```bash
# Set API key
API_KEY="sk-proj-..."

# Test with 1-minute chunk
http --ignore-stdin --timeout 120 -f POST https://api.openai.com/v1/audio/transcriptions \
  "Authorization:Bearer $API_KEY" \
  file@/tmp/test_1min.mp3 \
  model=gpt-4o-transcribe-diarize \
  response_format=diarized_json \
  chunking_strategy=auto \
  temperature=0
```
