package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ── Gemini API Request/Response structs ────────────────────

// GeminiRequest is the request body sent to the Gemini API.
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents a single content block.
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a text part of the content.
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse is the response from the Gemini API.
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GeminiClient forwards clean prompts to the Gemini API.
type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
	endpoint   string
}

// NewGeminiClient creates a GeminiClient with a 30-second timeout.
func NewGeminiClient() *GeminiClient {
	apiKey := os.Getenv("GEMINI_API_KEY")
	return &GeminiClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
	}
}

// GeminiResult holds the parsed response from the Gemini API.
type GeminiResult struct {
	Text       string // the generated text
	TokensUsed int    // total tokens consumed (prompt + response)
}

// Generate sends a prompt to the Gemini API and returns the response.
// Uses context.WithTimeout for request-level deadline enforcement.
func (gc *GeminiClient) Generate(ctx context.Context, prompt string) (*GeminiResult, error) {
	if gc.apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	// Build request body via struct → json.Marshal (not string concat)
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request with context timeout
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s?key=%s", gc.endpoint, gc.apiKey)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := gc.httpClient.Do(req)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("gemini request timed out after 30s")
		}
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API-level error
	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini API error (%d): %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	// Extract text from response candidates
	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned empty response")
	}

	return &GeminiResult{
		Text:       geminiResp.Candidates[0].Content.Parts[0].Text,
		TokensUsed: geminiResp.UsageMetadata.TotalTokenCount,
	}, nil
}
