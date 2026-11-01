// Package llm defines the provider-agnostic LLM client contract and the
// environment-driven factory that selects a concrete provider.
package llm

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/talesmud/talesmud/pkg/service/groq"
	"github.com/talesmud/talesmud/pkg/service/llmopts"
	"github.com/talesmud/talesmud/pkg/service/ollama"
)

// Client is the contract every LLM provider must satisfy. Implementations
// send a system prompt and a user prompt to a chat-completions API and
// return the assistant's text content.
type Client interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, error)
}

// Options is the additive per-request generation knob set. It is an alias of
// llmopts.Options so providers can implement CompleteWith without importing
// this package (which would create an import cycle).
type Options = llmopts.Options

// ClientWithOptions is the optional extension implemented by providers that
// honor a per-request max_tokens cap. Complete falls back to the legacy
// Client.Complete when a provider does not implement it, so the contract
// stays backward compatible.
type ClientWithOptions interface {
	Client
	CompleteWith(ctx context.Context, systemPrompt, userPrompt string, opts Options) (string, error)
}

// ErrNotConfigured is returned by Complete when the client is nil, i.e. the
// provider is disabled (LLM_PROVIDER=none/off/unknown).
var ErrNotConfigured = errors.New("llm: not configured")

// ModelInfo is an optional provider capability used for logs and cache keys.
type ModelInfo interface {
	Model() string
}

// ModelOf returns the provider model name when the client reports one.
// Unknown or nil clients return an empty string.
func ModelOf(c Client) string {
	if m, ok := c.(ModelInfo); ok && m != nil {
		return m.Model()
	}
	return ""
}

// Complete is the extended entry point. It uses CompleteWith when the client
// implements ClientWithOptions (varying temperature and max_tokens) and
// otherwise delegates to the legacy Complete with just the temperature, so
// existing providers and callers keep working unchanged.
func Complete(ctx context.Context, c Client, systemPrompt, userPrompt string, opts Options) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	if oc, ok := c.(ClientWithOptions); ok {
		return oc.CompleteWith(ctx, systemPrompt, userPrompt, opts)
	}
	return c.Complete(ctx, systemPrompt, userPrompt, opts.Temperature)
}

// Compile-time checks that both provider implementations satisfy the
// contract. The assertions live here (not in the groq package) because
// llm imports the providers, so the reverse would create an import cycle.
var (
	_ Client            = (*groq.Client)(nil)
	_ Client            = (*ollama.Client)(nil)
	_ ClientWithOptions = (*groq.Client)(nil)
	_ ClientWithOptions = (*ollama.Client)(nil)
	_ ModelInfo         = (*groq.Client)(nil)
	_ ModelInfo         = (*ollama.Client)(nil)
)

// NewFromEnv builds the LLM client selected by the LLM_PROVIDER variable.
//
// Selection rules:
//
//   - LLM_PROVIDER=ollama  → local Ollama client (the default provider).
//   - LLM_PROVIDER=groq    → Groq client, only when GROQ_API_KEY is present;
//     otherwise nil (generation disabled).
//   - LLM_PROVIDER=none or "off", or explicitly set to the empty string → nil.
//   - Unknown provider names → nil (generation disabled; callers treat a nil
//     Client as "not configured").
//   - LLM_PROVIDER unset (backward compatibility with pre-provider builds):
//     when GROQ_API_KEY is present the behavior is identical to the old
//     Groq-only wiring (Groq client); when it is absent the local Ollama
//     provider is selected so AI generation works out of the box against a
//     local Ollama daemon. This decision is deliberate: the old code path
//     (groq.NewClient) returned nil without a key, so key-less deployments
//     were disabled before; defaulting them to Ollama restores functionality
//     without changing any key-based deployment.
//
// A nil return means "AI generation disabled", not an error. Handlers must
// check for nil before calling Complete (a nil interface, never a typed nil
// pointer: the Groq key is verified before constructing the client).
func NewFromEnv() Client {
	raw, defined := os.LookupEnv("LLM_PROVIDER")
	provider := strings.ToLower(strings.TrimSpace(raw))
	groqKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))

	if defined && provider == "" {
		// Explicit empty value means "off".
		return nil
	}
	if !defined {
		// Backward compatibility: no provider configured. A Groq key keeps
		// the legacy behavior; otherwise fall back to local Ollama.
		if groqKey != "" {
			return groq.NewClient()
		}
		return ollama.NewClient()
	}

	switch provider {
	case "ollama":
		return ollama.NewClient()
	case "groq":
		if groqKey == "" {
			return nil
		}
		return groq.NewClient()
	case "none", "off":
		return nil
	default:
		// Unknown provider: disabled rather than a guessed fallback.
		return nil
	}
}
