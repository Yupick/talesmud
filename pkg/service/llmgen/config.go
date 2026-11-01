package llmgen

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config tunes pipeline behaviour. Zero values fall back to defaults
// (see ConfigFromEnv).
type Config struct {
	MaxAttempts        int
	PromptTimeout      time.Duration
	Cache              *Cache
	CacheTTL           map[Task]time.Duration
	Limiter            Limiter
	MaxConcurrent      int
	SemaphoreWait      time.Duration
	GlobalTimeout      time.Duration
	FallbackFuncs      map[Task]FallbackFunc
	AllowDeterministic bool
}

// ConfigFromEnv reads the subset of tuning knobs exposed to operators.
func ConfigFromEnv() Config {
	cfg := Config{
		MaxAttempts:   3,
		PromptTimeout: 20 * time.Second,
		MaxConcurrent: 2,
		SemaphoreWait: 2 * time.Second,
		Cache:         NewCache(envInt("LLM_CACHE_SIZE", 512)),
		CacheTTL: map[Task]time.Duration{
			TaskCharacterName:        30 * time.Minute,
			TaskCharacterDescription: 30 * time.Minute,
			TaskQuestGenerate:        30 * time.Minute,
			TaskNPCDialogue:          30 * time.Minute,
			TaskNPCMemorySummary:     0,
			TaskRoomDescription:      30 * time.Minute,
			TaskDMNarration:          30 * time.Minute,
		},
		Limiter:            NewWindowLimiter(envInt("LLM_RATE_PER_HOUR", 10), time.Hour),
		GlobalTimeout:      60 * time.Second,
		AllowDeterministic: envBool("LLM_ALLOW_DETERMINISTIC", true),
	}
	return cfg
}

func (c Config) withDefaults() Config {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 2
	}
	if c.SemaphoreWait <= 0 {
		c.SemaphoreWait = 2 * time.Second
	}
	if c.Cache == nil {
		c.Cache = NewCache(512)
	}
	if c.CacheTTL == nil {
		c.CacheTTL = map[Task]time.Duration{}
	}
	if c.Limiter == nil {
		c.Limiter = NewWindowLimiter(envInt("LLM_RATE_PER_HOUR", 10), time.Hour)
	}
	return c
}

func (c Config) cacheTTL(task Task) time.Duration {
	if c.CacheTTL == nil {
		return 0
	}
	return c.CacheTTL[task]
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
