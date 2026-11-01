package llmgen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CleanTextOutput normalizes a plain-text model reply: it trims whitespace,
// strips matching wrapping quotes and code fences, and truncates to maxRunes
// (0 means no truncation).
func CleanTextOutput(raw string, maxRunes int) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			first := strings.TrimSpace(s[:idx])
			if first != "" && !strings.ContainsAny(first, " \t{[\"") {
				// leading fence language tag (e.g. "json")
				s = s[idx+1:]
			}
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimSpace(s)
	if maxRunes > 0 {
		runes := []rune(s)
		if len(runes) > maxRunes {
			s = strings.TrimSpace(string(runes[:maxRunes]))
		}
	}
	return s
}

// ExtractJSON pulls the first balanced JSON object or array out of raw,
// tolerating surrounding prose and code fences, and returns its bytes.
func ExtractJSON(raw string) (json.RawMessage, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("llmgen: empty model output")
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			first := strings.TrimSpace(s[:idx])
			if !strings.ContainsAny(first, "{[") {
				s = s[idx+1:]
			}
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return nil, fmt.Errorf("llmgen: no JSON found in model output")
	}
	open := s[start]
	var closeCh byte
	switch open {
	case '{':
		closeCh = '}'
	case '[':
		closeCh = ']'
	default:
		return nil, fmt.Errorf("llmgen: unexpected JSON start %q", open)
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case closeCh:
			depth--
			if depth == 0 {
				candidate := s[start : i+1]
				if !json.Valid([]byte(candidate)) {
					return nil, fmt.Errorf("llmgen: extracted JSON is invalid")
				}
				return json.RawMessage(candidate), nil
			}
		}
	}
	return nil, fmt.Errorf("llmgen: unbalanced JSON in model output")
}
