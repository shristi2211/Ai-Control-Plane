package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ── JSON Response Types ────────────────────────────────────

// ErrorResponse is the consistent JSON error envelope for all errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ProxyRequest is the expected JSON body on POST /proxy.
type ProxyRequest struct {
	Prompt string `json:"prompt"`
}

// ProxyResponse is the JSON body returned on success.
type ProxyResponse struct {
	Service    string `json:"service"`
	Status     string `json:"status"`
	AIResponse string `json:"ai_response"`
	TokensUsed int    `json:"tokens_used"`
	PIIMasked  bool   `json:"pii_masked"`
	Timestamp  string `json:"timestamp"`
}

// ── Global dependencies (initialized once in main) ────────

var (
	quotaManager *QuotaManager
	auditLogger  *AuditLogger
	geminiClient *GeminiClient
)

func main() {
	port := getEnvDefault("PORT", "8080")

	// Initialize dependencies
	quotaManager = NewQuotaManager()
	auditLogger = NewAuditLogger(quotaManager.client)
	geminiClient = NewGeminiClient()

	mux := http.NewServeMux()

	// Health check — used by Docker & Kong
	mux.HandleFunc("/health", handleHealth)

	// Main proxy endpoint — full middleware chain
	mux.HandleFunc("/proxy", chainMiddleware(
		handleProxy,
		authMiddleware,
		quotaMiddleware,
	))

	// Catch-all info endpoint
	mux.HandleFunc("/", handleCatchAll)

	log.Printf("🚀 ai-proxy listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// ── Middleware Chain ──────────────────────────────────────

type middleware func(http.HandlerFunc) http.HandlerFunc

// chainMiddleware applies middlewares in reverse order so they execute
// in the order they are listed: first listed = outermost wrapper.
func chainMiddleware(handler http.HandlerFunc, middlewares ...middleware) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// authMiddleware checks for the X-User-ID header.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "Authentication Required",
				"X-User-ID header is missing. Please provide a valid user identifier.")
			return
		}
		next(w, r)
	}
}

// quotaMiddleware checks the user's daily token quota via Redis.
func quotaMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		ctx := r.Context()

		remaining, err := quotaManager.CheckQuota(ctx, userID)
		if err != nil {
			log.Printf("[QUOTA ERROR] %v", err)
			// Fail open — allow request if Redis is down
			next(w, r)
			return
		}

		if remaining <= 0 {
			writeError(w, http.StatusTooManyRequests, "Quota Exceeded",
				fmt.Sprintf("Daily limit of %d tokens reached. Please try again tomorrow.", quotaManager.GetDailyLimit()))
			return
		}

		next(w, r)
	}
}

// ── Route Handlers ───────────────────────────────────────

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service":   "ai-proxy",
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func handleCatchAll(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service":   "ai-proxy",
		"status":    "ok",
		"message":   "AI Proxy — Security Middleware Layer. Use POST /proxy to send prompts.",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleProxy is the main proxy handler — runs the full security pipeline.
func handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method Not Allowed",
			"Only POST requests are accepted on /proxy.")
		return
	}

	userID := r.Header.Get("X-User-ID")
	ctx := r.Context()

	// ── Guard: limit request body to 1 MB to prevent OOM ──
	const maxBodySize = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	// ── Parse request body ─────────────────────────────
	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "Payload Too Large",
				"Request body exceeds the 1 MB limit. Please shorten your prompt.")
			return
		}
		writeError(w, http.StatusBadRequest, "Invalid Request Body",
			"Request body must be valid JSON with a 'prompt' field.")
		return
	}

	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "Empty Prompt",
			"The 'prompt' field cannot be empty.")
		return
	}

	originalPrompt := req.Prompt

	// ── Step A: PII Scrubbing ──────────────────────────
	scrubResult := ScrubPII(originalPrompt)

	// ── Step B: Injection Check ────────────────────────
	injectionResult := CheckInjection(scrubResult.CleanedText)
	if injectionResult.IsInjection {
		// Log the blocked attempt
		auditLogger.Log(AuditEntry{
			UserID:            userID,
			PromptHash:        HashPrompt(originalPrompt),
			ScrubedPrompt:     scrubResult.CleanedText,
			PIIFound:          scrubResult.PIIFound,
			PIITypes:          scrubResult.PIITypes,
			InjectionDetected: true,
			InjectionPhrases:  injectionResult.MatchedPhrases,
			StatusCode:        http.StatusForbidden,
		})

		writeError(w, http.StatusForbidden, "Prompt Injection Detected",
			fmt.Sprintf("Your prompt was blocked because it contains potentially harmful instructions. Risk score: %d.", injectionResult.RiskScore))
		return
	}

	// ── Step C: Forward to Gemini ──────────────────────
	geminiResult, err := geminiClient.Generate(ctx, scrubResult.CleanedText)
	if err != nil {
		statusCode := http.StatusBadGateway
		errMsg := "AI service is currently unavailable."

		if ctx.Err() == context.DeadlineExceeded {
			statusCode = http.StatusGatewayTimeout
			errMsg = "AI service timed out. Please try a shorter prompt."
		}

		auditLogger.Log(AuditEntry{
			UserID:            userID,
			PromptHash:        HashPrompt(originalPrompt),
			ScrubedPrompt:     scrubResult.CleanedText,
			PIIFound:          scrubResult.PIIFound,
			PIITypes:          scrubResult.PIITypes,
			InjectionDetected: false,
			StatusCode:        statusCode,
		})

		log.Printf("[GEMINI ERROR] %v", err)
		writeError(w, statusCode, "AI Service Error", errMsg)
		return
	}

	// ── Step D: Response Guardrails ────────────────────
	guardrailResult := ValidateResponse(geminiResult.Text)

	// ── Step E: Consume tokens from quota ──────────────
	if err := quotaManager.ConsumeTokens(ctx, userID, geminiResult.TokensUsed); err != nil {
		log.Printf("[QUOTA ERROR] failed to consume tokens: %v", err)
		// Non-fatal — we still return the response
	}

	// ── Step F: Audit Log ──────────────────────────────
	auditLogger.Log(AuditEntry{
		UserID:            userID,
		PromptHash:        HashPrompt(originalPrompt),
		ScrubedPrompt:     scrubResult.CleanedText,
		PIIFound:          scrubResult.PIIFound,
		PIITypes:          scrubResult.PIITypes,
		InjectionDetected: false,
		ResponseFiltered:  guardrailResult.IsFiltered,
		StatusCode:        http.StatusOK,
		TokensUsed:        geminiResult.TokensUsed,
	})

	// ── Return response ────────────────────────────────
	writeJSON(w, http.StatusOK, ProxyResponse{
		Service:    "ai-proxy",
		Status:     "ok",
		AIResponse: guardrailResult.CleanedResponse,
		TokensUsed: geminiResult.TokensUsed,
		PIIMasked:  scrubResult.PIIFound,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

// ── Helpers ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errTitle, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   errTitle,
		Code:    status,
		Message: message,
	})
}
