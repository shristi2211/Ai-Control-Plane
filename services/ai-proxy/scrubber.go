package main

import (
	"regexp"
)

// scrubPattern pairs a pre-compiled regex with its replacement mask.
type scrubPattern struct {
	regex       *regexp.Regexp
	replacement string
	label       string // human-readable name for audit logs
}

// All regex patterns are pre-compiled at package init time (not per-request).
// Word boundaries (\b) prevent false positives on normal numbers.
var piiPatterns = []scrubPattern{
	{
		regex:       regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		replacement: "[MASKED_EMAIL]",
		label:       "email",
	},
	{
		regex:       regexp.MustCompile(`\b\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{1,4}\b`),
		replacement: "[MASKED_CC]",
		label:       "credit_card",
	},
	{
		regex:       regexp.MustCompile(`\b(\+?\d{1,3}[\-.\s]?)?\(?\d{3}\)?[\-.\s]?\d{3}[\-.\s]?\d{4}\b`),
		replacement: "[MASKED_PHONE]",
		label:       "phone",
	},
	{
		regex:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		replacement: "[MASKED_SSN]",
		label:       "ssn",
	},
	{
		regex:       regexp.MustCompile(`\b\d{4}\s\d{4}\s\d{4}\b`),
		replacement: "[MASKED_AADHAAR]",
		label:       "aadhaar",
	},
}

// ScrubResult holds the cleaned text and metadata about what was found.
type ScrubResult struct {
	CleanedText string   // prompt after masking
	PIIFound    bool     // true if any pattern matched
	PIITypes    []string // list of matched pattern labels
}

// ScrubPII scans the input text for sensitive data patterns
// and replaces them with mask tokens. Returns metadata about what was found.
func ScrubPII(text string) ScrubResult {
	result := ScrubResult{CleanedText: text}

	for _, p := range piiPatterns {
		if p.regex.MatchString(result.CleanedText) {
			result.PIIFound = true
			result.PIITypes = append(result.PIITypes, p.label)
			result.CleanedText = p.regex.ReplaceAllString(result.CleanedText, p.replacement)
		}
	}

	return result
}
