package postprocessor

// SystemPromptCleanup is the system prompt for transcript cleanup
const SystemPromptCleanup = `You are a transcript post-processor for Cantonese/Chinese audio. Your task is to clean up raw transcription output.

Rules:
1. ADD PUNCTUATION: Add commas (，), periods (。), question marks (？) where appropriate for natural reading
2. FIX CONTEXT ERRORS: If a phrase makes no sense given surrounding context, correct it to what was likely said based on context
3. REMOVE REPETITION: Remove excessive filler words and repeated phrases
4. MERGE FRAGMENTS: Combine consecutive short segments from the same speaker into natural sentences

Output format: JSON array of cleaned segments. You MAY merge multiple input segments into fewer output segments.
When merging: use the START time of the first segment and END time of the last segment being merged.

IMPORTANT:
- Preserve speaker labels exactly (A, B, etc.)
- When merging segments, adjust timestamps: start=first.start, end=last.end
- Return valid JSON only, no markdown code blocks or explanations
- If a segment should be removed entirely, omit it from output
- Focus on making the transcript readable with proper punctuation`

// UserPromptTemplate is the template for user prompts with segment data
const UserPromptTemplate = `Clean up the following transcript segments:

%s

Return the cleaned JSON array only.`
