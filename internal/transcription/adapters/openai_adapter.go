package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"scriberr/internal/transcription/interfaces"
	"scriberr/internal/transcription/splitter"
	"scriberr/pkg/logger"
)

// OpenAIAdapter implements the TranscriptionAdapter interface for OpenAI API
type OpenAIAdapter struct {
	*BaseAdapter
	apiKey string
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(apiKey string) *OpenAIAdapter {
	capabilities := interfaces.ModelCapabilities{
		ModelID:     "openai_whisper",
		ModelFamily: "openai",
		DisplayName: "OpenAI Whisper API",
		Description: "Cloud-based transcription using OpenAI's Whisper model",
		Version:     "v1",
		SupportedLanguages: []string{
			"af", "ar", "hy", "az", "be", "bs", "bg", "ca", "zh", "hr", "cs", "da", "nl", "en", "et", "fi", "fr", "gl", "de", "el", "he", "hi", "hu", "is", "id", "it", "ja", "kn", "kk", "ko", "lv", "lt", "mk", "ms", "mr", "mi", "ne", "no", "fa", "pl", "pt", "ro", "ru", "sr", "sk", "sl", "es", "sw", "sv", "tl", "ta", "th", "tr", "uk", "ur", "vi", "cy",
		},
		SupportedFormats:  []string{"flac", "mp3", "mp4", "mpeg", "mpga", "m4a", "ogg", "wav", "webm"},
		RequiresGPU:       false,
		MemoryRequirement: 0, // Cloud-based
		Features: map[string]bool{
			"timestamps":         true,  // Verbose JSON response includes segments
			"word_level":         false, // Not supported by standard API yet (unless using verbose_json with timestamp_granularities which is beta)
			"diarization":        false, // Not supported by OpenAI API
			"translation":        true,
			"language_detection": true,
			"vad":                true, // Implicit
		},
		Metadata: map[string]string{
			"provider": "openai",
			"api_url":  "https://api.openai.com/v1/audio/transcriptions",
		},
	}

	schema := []interfaces.ParameterSchema{
		{
			Name:        "api_key",
			Type:        "string",
			Required:    false, // Can be provided in config
			Description: "OpenAI API Key (overrides system default)",
			Group:       "authentication",
		},
		{
			Name:        "model",
			Type:        "string",
			Required:    false,
			Default:     "gpt-4o-transcribe",
			Options:     []string{"whisper-1", "gpt-4o-transcribe", "gpt-4o-mini-transcribe", "gpt-4o-transcribe-diarize"},
			Description: "ID of the model to use (gpt-4o-transcribe-diarize for speaker separation)",
			Group:       "basic",
		},
		{
			Name:        "language",
			Type:        "string",
			Required:    false,
			Description: "Language of the input audio (ISO-639-1)",
			Group:       "basic",
		},
		{
			Name:        "prompt",
			Type:        "string",
			Required:    false,
			Description: "Optional text to guide the model's style or continue a previous audio segment",
			Group:       "advanced",
		},
		{
			Name:        "temperature",
			Type:        "float",
			Required:    false,
			Default:     0.0,
			Min:         &[]float64{0.0}[0],
			Max:         &[]float64{1.0}[0],
			Description: "Sampling temperature",
			Group:       "quality",
		},
	}

	baseAdapter := NewBaseAdapter("openai_whisper", "", capabilities, schema)

	return &OpenAIAdapter{
		BaseAdapter: baseAdapter,
		apiKey:      apiKey,
	}
}

// GetSupportedModels returns the list of OpenAI models supported
func (a *OpenAIAdapter) GetSupportedModels() []string {
	return []string{"whisper-1", "gpt-4o-transcribe", "gpt-4o-mini-transcribe", "gpt-4o-transcribe-diarize"}
}

// PrepareEnvironment is a no-op for cloud adapters
func (a *OpenAIAdapter) PrepareEnvironment(ctx context.Context) error {
	a.initialized = true
	return nil
}

// Transcribe processes audio using OpenAI API
//
//nolint:gocyclo // API interaction involves many steps
func (a *OpenAIAdapter) Transcribe(ctx context.Context, input interfaces.AudioInput, params map[string]interface{}, procCtx interfaces.ProcessingContext) (*interfaces.TranscriptResult, error) {
	startTime := time.Now()
	a.LogProcessingStart(input, procCtx)
	defer func() {
		a.LogProcessingEnd(procCtx, time.Since(startTime), nil)
	}()

	// Helper to write to job log file
	writeLog := func(format string, args ...interface{}) {
		logPath := filepath.Join(procCtx.OutputDirectory, "transcription.log")
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Failed to open log file", "path", logPath, "error", err)
			return
		}
		defer f.Close()

		msg := fmt.Sprintf(format, args...)
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
	}

	writeLog("Starting OpenAI transcription for job %s", procCtx.JobID)
	writeLog("Input file: %s", input.FilePath)

	// Validate input
	if err := a.ValidateAudioInput(input); err != nil {
		writeLog("Error: Invalid audio input: %v", err)
		return nil, fmt.Errorf("invalid audio input: %w", err)
	}

	// Get API Key
	apiKey := a.apiKey
	if key, ok := params["api_key"].(string); ok && key != "" {
		apiKey = key
	}

	if apiKey == "" {
		writeLog("Error: OpenAI API key is required but not provided")
		return nil, fmt.Errorf("OpenAI API key is required but not provided")
	}

	// Prepare request body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	file, err := os.Open(input.FilePath)
	if err != nil {
		writeLog("Error: Failed to open audio file: %v", err)
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(input.FilePath))
	if err != nil {
		writeLog("Error: Failed to create form file: %v", err)
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		writeLog("Error: Failed to copy file content: %v", err)
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add parameters
	model := a.GetStringParameter(params, "model")
	if model == "" {
		model = "whisper-1"
	}
	writeLog("Model: %s", model)
	_ = writer.WriteField("model", model)

	if strings.HasPrefix(model, "gpt-4o") {
		if strings.Contains(model, "diarize") {
			_ = writer.WriteField("response_format", "diarized_json")
			// chunking_strategy is required for diarization models
			_ = writer.WriteField("chunking_strategy", "auto")
		} else {
			_ = writer.WriteField("response_format", "json")
		}
		// gpt-4o models don't support timestamp_granularities with these formats
	} else {
		_ = writer.WriteField("response_format", "verbose_json")
		// timestamp_granularities is only supported for whisper-1
		if model == "whisper-1" {
			_ = writer.WriteField("timestamp_granularities[]", "word")    // Request word timestamps
			_ = writer.WriteField("timestamp_granularities[]", "segment") // Request segment timestamps
		}
	}

	if lang := a.GetStringParameter(params, "language"); lang != "" {
		writeLog("Language: %s", lang)
		_ = writer.WriteField("language", lang)
	}

	if prompt := a.GetStringParameter(params, "prompt"); prompt != "" {
		writeLog("Prompt provided")
		_ = writer.WriteField("prompt", prompt)
	}

	temp := a.GetFloatParameter(params, "temperature")
	writeLog("Temperature: %.2f", temp)
	_ = writer.WriteField("temperature", fmt.Sprintf("%.2f", temp))

	// Add known_speaker_references for cross-chunk speaker consistency
	// OpenAI API expects indexed array format: known_speaker_names[0], known_speaker_names[1]
	// NOT PHP-style array notation with [] suffix
	if refs, ok := params["known_speaker_references"]; ok {
		if speakerRefs, ok := refs.([]splitter.SpeakerReference); ok && len(speakerRefs) > 0 {
			writeLog("Adding %d speaker references for cross-chunk consistency", len(speakerRefs))
			for i, ref := range speakerRefs {
				dataURLLen := len(ref.ReferenceAudio)
				writeLog("Speaker reference [%d]: speaker=%s, data_url_length=%d bytes", i, ref.Speaker, dataURLLen)

				// Validate data URL format and size
				if dataURLLen > 1000000 { // 1MB limit for safety
					writeLog("Warning: Speaker reference data URL is very large (%d bytes), may cause API rejection", dataURLLen)
				}
				if !strings.HasPrefix(ref.ReferenceAudio, "data:audio/") {
					writeLog("Warning: Speaker reference does not start with 'data:audio/', format may be incorrect")
				}

				// Use indexed notation: known_speaker_names[0], known_speaker_references[0]
				_ = writer.WriteField(fmt.Sprintf("known_speaker_names[%d]", i), ref.Speaker)
				_ = writer.WriteField(fmt.Sprintf("known_speaker_references[%d]", i), ref.ReferenceAudio)
			}
		}
	}

	if err := writer.Close(); err != nil {
		writeLog("Error: Failed to close multipart writer: %v", err)
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	writeLog("Sending request to OpenAI API...")
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", body)
	if err != nil {
		writeLog("Error: Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Execute request with retry logic for transient network errors
	// Force HTTP/1.1 to avoid HTTP/2 framing layer issues with OpenAI's API
	// during long-running transcription requests
	client := &http.Client{
		Timeout: 10 * time.Minute, // Generous timeout for large files
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		},
	}

	var resp *http.Response
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		writeLog("Attempt %d/%d: Sending request (file size: %d bytes)...", attempt, maxRetries, body.Len())
		resp, err = client.Do(req)
		if err == nil {
			writeLog("Attempt %d/%d: Response received (status: %d)", attempt, maxRetries, resp.StatusCode)
			break // Success
		}

		// Log detailed error information
		writeLog("Attempt %d/%d: Request error: %v (type: %T)", attempt, maxRetries, err, err)

		// Check if error is retryable (network errors, EOF, timeouts)
		errStr := err.Error()
		isRetryable := strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "network is unreachable") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "connection closed")

		if !isRetryable || attempt == maxRetries {
			writeLog("Error: Request failed after %d attempts: %v", attempt, err)
			writeLog("Error details - Retryable: %v, Attempt: %d, MaxRetries: %d", isRetryable, attempt, maxRetries)
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Wait before retry with exponential backoff
		backoff := time.Duration(attempt*attempt) * 5 * time.Second
		writeLog("Request failed (attempt %d/%d): %v. Retrying in %v...", attempt, maxRetries, err, backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		// Re-read file and recreate request for retry
		file.Seek(0, 0)
		body.Reset()
		writer = multipart.NewWriter(body)

		part, err = writer.CreateFormFile("file", filepath.Base(input.FilePath))
		if err != nil {
			writeLog("Error: Failed to create form file on retry: %v", err)
			return nil, fmt.Errorf("failed to create form file on retry: %w", err)
		}
		if _, err = io.Copy(part, file); err != nil {
			writeLog("Error: Failed to copy file content on retry: %v", err)
			return nil, fmt.Errorf("failed to copy file content on retry: %w", err)
		}

		_ = writer.WriteField("model", model)
		if strings.HasPrefix(model, "gpt-4o") {
			if strings.Contains(model, "diarize") {
				_ = writer.WriteField("response_format", "diarized_json")
				_ = writer.WriteField("chunking_strategy", "auto")
			} else {
				_ = writer.WriteField("response_format", "json")
			}
		} else {
			_ = writer.WriteField("response_format", "verbose_json")
			if model == "whisper-1" {
				_ = writer.WriteField("timestamp_granularities[]", "word")
				_ = writer.WriteField("timestamp_granularities[]", "segment")
			}
		}
		if lang := a.GetStringParameter(params, "language"); lang != "" {
			_ = writer.WriteField("language", lang)
		}
		if prompt := a.GetStringParameter(params, "prompt"); prompt != "" {
			_ = writer.WriteField("prompt", prompt)
		}
		_ = writer.WriteField("temperature", fmt.Sprintf("%.2f", temp))
		// Re-add speaker references on retry
		if refs, ok := params["known_speaker_references"]; ok {
			if speakerRefs, ok := refs.([]splitter.SpeakerReference); ok && len(speakerRefs) > 0 {
				writeLog("Re-adding %d speaker references on retry", len(speakerRefs))
				for i, ref := range speakerRefs {
					dataURLLen := len(ref.ReferenceAudio)
					writeLog("Speaker reference [%d]: speaker=%s, data_url_length=%d bytes", i, ref.Speaker, dataURLLen)

					if dataURLLen > 1000000 {
						writeLog("Warning: Speaker reference data URL is very large (%d bytes), may cause API rejection", dataURLLen)
					}
					if !strings.HasPrefix(ref.ReferenceAudio, "data:audio/") {
						writeLog("Warning: Speaker reference does not start with 'data:audio/', format may be incorrect")
					}

					// Use indexed notation
					_ = writer.WriteField(fmt.Sprintf("known_speaker_names[%d]", i), ref.Speaker)
					_ = writer.WriteField(fmt.Sprintf("known_speaker_references[%d]", i), ref.ReferenceAudio)
				}
			}
		}
		writer.Close()

		req, err = http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", body)
		if err != nil {
			writeLog("Error: Failed to create request on retry: %v", err)
			return nil, fmt.Errorf("failed to create request on retry: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		writeLog("Error: OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	writeLog("Response received. Parsing...")

	// Read response body for flexible parsing
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeLog("Error: Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result *interfaces.TranscriptResult

	// Handle diarized response format (gpt-4o-transcribe-diarize)
	if strings.Contains(model, "diarize") {
		var diarizedResponse struct {
			Text     string `json:"text"`
			Segments []struct {
				ID      string  `json:"id"`
				Type    string  `json:"type"`
				Start   float64 `json:"start"`
				End     float64 `json:"end"`
				Text    string  `json:"text"`
				Speaker string  `json:"speaker"`
			} `json:"segments"`
			Usage struct {
				TotalTokens int `json:"total_tokens"`
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(respBody, &diarizedResponse); err != nil {
			writeLog("Error: Failed to decode diarized response: %v", err)
			return nil, fmt.Errorf("failed to decode diarized response: %w", err)
		}

		writeLog("Diarized transcription completed. Segments: %d", len(diarizedResponse.Segments))

		result = &interfaces.TranscriptResult{
			Text:           diarizedResponse.Text,
			Segments:       make([]interfaces.TranscriptSegment, len(diarizedResponse.Segments)),
			ProcessingTime: time.Since(startTime),
			ModelUsed:      model,
			Metadata:       a.CreateDefaultMetadata(params),
		}

		for i, seg := range diarizedResponse.Segments {
			speaker := seg.Speaker
			result.Segments[i] = interfaces.TranscriptSegment{
				Start:   seg.Start,
				End:     seg.End,
				Text:    seg.Text,
				Speaker: &speaker,
			}
		}
	} else {
		// Handle standard response format (whisper-1, gpt-4o-transcribe)
		var openAIResponse struct {
			Task     string  `json:"task"`
			Language string  `json:"language"`
			Duration float64 `json:"duration"`
			Text     string  `json:"text"`
			Segments []struct {
				ID               int     `json:"id"`
				Seek             int     `json:"seek"`
				Start            float64 `json:"start"`
				End              float64 `json:"end"`
				Text             string  `json:"text"`
				Tokens           []int   `json:"tokens"`
				Temperature      float64 `json:"temperature"`
				AvgLogprob       float64 `json:"avg_logprob"`
				CompressionRatio float64 `json:"compression_ratio"`
				NoSpeechProb     float64 `json:"no_speech_prob"`
			} `json:"segments"`
			Words []struct {
				Word  string  `json:"word"`
				Start float64 `json:"start"`
				End   float64 `json:"end"`
			} `json:"words"`
		}

		if err := json.Unmarshal(respBody, &openAIResponse); err != nil {
			writeLog("Error: Failed to decode response: %v", err)
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		writeLog("Transcription completed successfully. Duration: %.2fs, Words: %d", openAIResponse.Duration, len(openAIResponse.Words))

		result = &interfaces.TranscriptResult{
			Language:       openAIResponse.Language,
			Text:           openAIResponse.Text,
			Segments:       make([]interfaces.TranscriptSegment, len(openAIResponse.Segments)),
			WordSegments:   make([]interfaces.TranscriptWord, len(openAIResponse.Words)),
			ProcessingTime: time.Since(startTime),
			ModelUsed:      model,
			Metadata:       a.CreateDefaultMetadata(params),
		}

		if len(openAIResponse.Segments) > 0 {
			for i, seg := range openAIResponse.Segments {
				result.Segments[i] = interfaces.TranscriptSegment{
					Start: seg.Start,
					End:   seg.End,
					Text:  seg.Text,
				}
			}
		} else if openAIResponse.Text != "" {
			// If no segments returned (e.g. standard json format), create one segment with the whole text
			result.Segments = []interfaces.TranscriptSegment{
				{
					Start: 0,
					End:   openAIResponse.Duration,
					Text:  openAIResponse.Text,
				},
			}
		}

		for i, word := range openAIResponse.Words {
			result.WordSegments[i] = interfaces.TranscriptWord{
				Word:  word.Word,
				Start: word.Start,
				End:   word.End,
			}
		}
	}

	return result, nil
}

// GetEstimatedProcessingTime provides OpenAI-specific time estimation
func (a *OpenAIAdapter) GetEstimatedProcessingTime(input interfaces.AudioInput) time.Duration {
	// Cloud transcription is generally faster, approx 10-20% of audio duration
	audioDuration := input.Duration
	if audioDuration == 0 {
		return 30 * time.Second // Fallback
	}
	return time.Duration(float64(audioDuration) * 0.15)
}
