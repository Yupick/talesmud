// Package llmopts holds the provider-agnostic generation options shared
// between the llm client contract and its concrete providers.
//
// It lives in its own leaf package to break an import cycle: pkg/service/llm
// imports the providers (ollama, groq) for the factory, so the providers
// cannot import pkg/service/llm back. Both sides depend on this package
// instead.
package llmopts

// Options are the per-request generation knobs. They are additive: a zero
// value means "provider default" (temperature 0 and the provider's 300-token
// cap), preserving the historic Complete behavior.
type Options struct {
	Temperature float64
	// MaxTokens caps the generated tokens. 0 falls back to the provider
	// default (300 for both Ollama and Groq).
	MaxTokens int
}
