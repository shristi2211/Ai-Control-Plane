package main

import (
	"regexp"
	"strings"
)

// Banned words/phrases that should never appear in AI responses.
var bannedWords = []string{
	"kill yourself",
	"how to make a bomb",
	"how to hack",
	"illegal drugs",
	"child exploitation",
	"self-harm instructions",
	"terrorist attack",
	"white supremacy",
	"hate speech",
}

// Hallucination patterns — pre-compiled regex for fake references.
var hallucinationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://(?:www\.)?fake[\w\-]*\.(?:com|org|net)`),
	regexp.MustCompile(`https?://(?:www\.)?example-source[\w\-]*\.(?:com|org)`),
	regexp.MustCompile(`(?i)according to a (?:recent )?study that (?:doesn't|does not) exist`),
	regexp.MustCompile(`(?i)I (?:just )?made (?:that|this) up`),
}

// GuardrailResult holds the outcome of response validation.
type GuardrailResult struct {
	IsFiltered         bool     // true if content was blocked/redacted
	BannedWordsFound   []string // which banned phrases were detected
	HallucinationFound bool     // true if hallucination patterns matched
	CleanedResponse    string   // the (potentially redacted) response text
}

// ValidateResponse checks the AI response for banned content and hallucinations.
// If violations are found, the response is redacted and flagged.
func ValidateResponse(responseText string) GuardrailResult {
	result := GuardrailResult{CleanedResponse: responseText}
	lower := strings.ToLower(responseText)

	// Check banned words
	for _, word := range bannedWords {
		if strings.Contains(lower, strings.ToLower(word)) {
			result.IsFiltered = true
			result.BannedWordsFound = append(result.BannedWordsFound, word)
		}
	}

	// Check hallucination patterns
	for _, pattern := range hallucinationPatterns {
		if pattern.MatchString(responseText) {
			result.IsFiltered = true
			result.HallucinationFound = true
			break
		}
	}

	// If filtered, redact the entire response for safety
	if result.IsFiltered {
		result.CleanedResponse = "[RESPONSE BLOCKED] The AI response was filtered because it contained potentially harmful or unreliable content. Please rephrase your query."
	}

	return result
}
