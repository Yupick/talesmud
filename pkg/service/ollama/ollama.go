// Package ollama implements the llm.Client contract for a local Ollama
// daemon through its OpenAI-compatible chat completions API.
//
// Configuration (all optional, read from the environment):
//
//	OLLAMA_BASE_URL     default http://localhost:11434
//	OLLAMA_MODEL        default llama3.1
//	LLM_TIMEOUT_SECONDS default 60 (local inference is slower than hosted APIs)
//
// No API key is required.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/talesmud/talesmud/pkg/service/llmopts"
)

const (
	// DefaultBaseURL is Ollama's default HTTP listener.
	DefaultBaseURL = "http://localhost:11434"
	// DefaultModel is used when OLLAMA_MODEL is not set.
	DefaultModel = "llama3.1"
	// defaultTimeout is used when LLM_TIMEOUT_SECONDS is unset or invalid.
	// Local inference is slower than hosted APIs such as Groq (15s), so the
	// local default is deliberately longer.
	defaultTimeout = 60 * time.Second
	// maxTokens mirrors the Groq client's generation cap.
	maxTokens = 300
)

// Message represents a single chat message in the OpenAI-compatible format.
// Structs are local to this package: providers must not import each other,
// so shared wire shapes are deliberately duplicated rather than coupled.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is the payload sent to Ollama's /v1/chat/completions endpoint.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Response is the parsed response from the Ollama API.
type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Client wraps communication with a local Ollama daemon.
type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// NewClient creates a client for a local Ollama daemon. It always returns a
// usable client (no credentials are required); callers that disable the
// provider entirely do so through llm.NewFromEnv, not this constructor.
func NewClient() *Client {
	baseURL := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
	if model == "" {
		model = DefaultModel
	}

	timeout := defaultTimeout
	if raw := strings.TrimSpace(os.Getenv("LLM_TIMEOUT_SECONDS")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		model:      model,
	}
}

// Complete sends a chat completion request and returns the text content.
// Errors wrap the underlying cause: transport failures (including timeout
// and context cancellation) come back as "ollama api call: ...", non-200
// statuses include the status code and body, and payloads without choices
// report "no choices".
func (c *Client) Complete(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, error) {
	return c.CompleteWith(ctx, systemPrompt, userPrompt, llmopts.Options{Temperature: temperature})
}

// CompleteWith is the additive extension that also varies max_tokens. A zero
// MaxTokens keeps the provider default (300).
func (c *Client) CompleteWith(ctx context.Context, systemPrompt, userPrompt string, opts llmopts.Options) (string, error) {
	limit := maxTokens
	if opts.MaxTokens > 0 {
		limit = opts.MaxTokens
	}
	req := Request{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: opts.Temperature,
		MaxTokens:   limit,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// The upstream body is deliberately not echoed: handlers return
		// err.Error() to HTTP clients, and provider bodies can carry request
		// details. The status code is enough to diagnose the failure.
		return "", fmt.Errorf("ollama api returned status %d", resp.StatusCode)
	}

	var ollamaResp Response
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(ollamaResp.Choices) == 0 {
		return "", fmt.Errorf("ollama api returned no choices")
	}

	return ollamaResp.Choices[0].Message.Content, nil
}

// Model reports the configured model name for logs and cache keys.
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}
