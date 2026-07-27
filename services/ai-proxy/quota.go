package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// QuotaManager handles per-user daily token usage limits via Redis.
type QuotaManager struct {
	client     *redis.Client
	dailyLimit int
}

// NewQuotaManager creates a QuotaManager connected to Redis.
func NewQuotaManager() *QuotaManager {
	host := getEnvDefault("REDIS_HOST", "localhost")
	port := getEnvDefault("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD")
	limitStr := getEnvDefault("DAILY_TOKEN_LIMIT", "10000")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10000
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})

	return &QuotaManager{
		client:     rdb,
		dailyLimit: limit,
	}
}

// quotaKey returns the Redis key for a user's daily usage.
func quotaKey(userID string) string {
	today := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("quota:%s:%s", userID, today)
}

// CheckQuota returns true if the user still has tokens remaining today.
// Returns (remaining tokens, error).
func (qm *QuotaManager) CheckQuota(ctx context.Context, userID string) (int, error) {
	key := quotaKey(userID)

	val, err := qm.client.Get(ctx, key).Int()
	if err == redis.Nil {
		// First request of the day — full quota available
		return qm.dailyLimit, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis get failed: %w", err)
	}

	remaining := qm.dailyLimit - val
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// ConsumeTokens increments the user's token usage for today.
// Sets a 24-hour TTL on the key if it's new.
func (qm *QuotaManager) ConsumeTokens(ctx context.Context, userID string, tokens int) error {
	key := quotaKey(userID)

	pipe := qm.client.Pipeline()
	pipe.IncrBy(ctx, key, int64(tokens))
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline failed: %w", err)
	}

	return nil
}

// GetDailyLimit returns the configured daily token limit.
func (qm *QuotaManager) GetDailyLimit() int {
	return qm.dailyLimit
}

// getEnvDefault returns the env var value or a fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
