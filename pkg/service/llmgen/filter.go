package llmgen

import (
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	suspiciousIDRe = regexp.MustCompile(`\b[Rr]\d{3,}\b|\bquest:[A-Za-z0-9_-]+|\bnpc:[A-Za-z0-9_-]+`)
	injectionRe    = regexp.MustCompile(`(?i)(ignore (all|previous|above) instructions|disregard .*instructions|system prompt|you are now|jailbreak|as an ai language model)`)
)

// OutputAllowed rejects empty, oversized or leaky model output before it is
// trusted. dm narration is capped and must not leak room/quest/npc ids.
func OutputAllowed(task Task, s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	if ContainsBlockedTerm(s) {
		return false
	}
	switch task {
	case TaskDMNarration:
		if len([]rune(s)) > 240 {
			return false
		}
		if suspiciousIDRe.MatchString(s) {
			return false
		}
	}
	return true
}

// LooksLikeInjection reports whether player-supplied text tries to hijack
// the model. It is advisory: the untrusted block already delimits input.
func LooksLikeInjection(s string) bool {
	return injectionRe.MatchString(s)
}

var (
	blockedOnce sync.Once
	blockedSet  map[string]struct{}
)

func loadBlockedTerms() {
	blockedSet = map[string]struct{}{}
	path := strings.TrimSpace(os.Getenv("LLM_BLOCKED_TERMS_FILE"))
	if path == "" {
		path = "config/llm_blocked_terms.txt"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		blockedSet[strings.ToLower(line)] = struct{}{}
	}
}

// ContainsBlockedTerm reports whether s includes any configured denylisted
// term (case-insensitive substring match).
func ContainsBlockedTerm(s string) bool {
	blockedOnce.Do(loadBlockedTerms)
	if len(blockedSet) == 0 {
		return false
	}
	lower := strings.ToLower(s)
	for term := range blockedSet {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}
