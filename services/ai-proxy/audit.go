package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// AuditEntry represents a single request/response audit record.
type AuditEntry struct {
	Timestamp         string   `json:"timestamp"`
	UserID            string   `json:"user_id"`
	PromptHash        string   `json:"prompt_hash"`
	ScrubedPrompt     string   `json:"scrubbed_prompt"`
	PIIFound          bool     `json:"pii_found"`
	PIITypes          []string `json:"pii_types,omitempty"`
	InjectionDetected bool     `json:"injection_detected"`
	InjectionPhrases  []string `json:"injection_phrases,omitempty"`
	ResponseFiltered  bool     `json:"response_filtered"`
	StatusCode        int      `json:"status_code"`
	TokensUsed        int      `json:"tokens_used"`
}

// AuditLogger writes structured compliance logs to stdout and Redis.
type AuditLogger struct {
	redisClient *redis.Client
}

// NewAuditLogger creates an AuditLogger using the provided Redis client.
func NewAuditLogger(redisClient *redis.Client) *AuditLogger {
	return &AuditLogger{redisClient: redisClient}
}

// HashPrompt returns a SHA-256 hex digest of the original (un-scrubbed) prompt.
// This allows compliance teams to identify requests without storing raw user data.
func HashPrompt(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h)
}

// Log writes the audit entry as structured JSON to stdout and pushes
// it to the Redis list "audit:log" for persistent storage.
func (al *AuditLogger) Log(entry AuditEntry) {
	// Fill timestamp if not already set
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[AUDIT ERROR] failed to marshal entry: %v", err)
		return
	}

	// Write to stdout (Docker log driver captures this)
	log.Printf("[AUDIT] %s", string(data))

	// Push to Redis list for persistence, capped at 10 000 entries
	if al.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		const auditKey = "audit:log"
		const maxAuditEntries int64 = 10000

		pipe := al.redisClient.Pipeline()
		pipe.RPush(ctx, auditKey, string(data))
		pipe.LTrim(ctx, auditKey, -maxAuditEntries, -1) // keep only the newest N
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("[AUDIT ERROR] redis push/trim failed: %v", err)
		}
	}
}
