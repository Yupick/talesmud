package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// chatServer returns a test server whose /v1/chat/completions
// endpoint is handled by fn. The base URL is exported through the
// OLLAMA_BASE_URL environment variable before NewClient runs.
func chatServer(t *testing.T, fn http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(fn)
	t.Cleanup(ts.Close)
	t.Setenv("OLLAMA_BASE_URL", ts.URL)
	return ts
}

func TestCompleteSuccess(t *testing.T) {
	var gotReq Request
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Thorin"}}]}`)
	})
	t.Setenv("OLLAMA_MODEL", "llama3.1")

	client := NewClient()
	content, err := client.Complete(context.Background(), "system prompt", "user prompt", 0.7)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if content != "Thorin" {
		t.Fatalf("content = %q, want %q", content, "Thorin")
	}

	if gotReq.Model != "llama3.1" {
		t.Errorf("request model = %q, want llama3.1", gotReq.Model)
	}
	if gotReq.Temperature != 0.7 {
		t.Errorf("request temperature = %v, want 0.7", gotReq.Temperature)
	}
	if len(gotReq.Messages) != 2 {
		t.Fatalf("request messages = %+v, want 2 entries", gotReq.Messages)
	}
	if gotReq.Messages[0].Role != "system" || gotReq.Messages[0].Content != "system prompt" {
		t.Errorf("system message = %+v", gotReq.Messages[0])
	}
	if gotReq.Messages[1].Role != "user" || gotReq.Messages[1].Content != "user prompt" {
		t.Errorf("user message = %+v", gotReq.Messages[1])
	}
	if gotReq.MaxTokens != maxTokens {
		t.Errorf("request max_tokens = %d, want %d", gotReq.MaxTokens, maxTokens)
	}
}

func TestCompleteHTTPErrorStatus(t *testing.T) {
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"boom"}`)
	})

	client := NewClient()
	_, err := client.Complete(context.Background(), "s", "u", 0.5)
	if err == nil {
		t.Fatal("Complete succeeded, want error on HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not contain status code 500", err)
	}
	// The upstream body must not reach the error (it is returned to HTTP clients).
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q leaks the upstream response body", err)
	}
}

func TestCompleteNoChoices(t *testing.T) {
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[]}`)
	})

	client := NewClient()
	_, err := client.Complete(context.Background(), "s", "u", 0.5)
	if err == nil {
		t.Fatal("Complete succeeded, want error for empty choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error %q does not mention missing choices", err)
	}
}

func TestCompleteClientTimeout(t *testing.T) {
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // longer than the client timeout below
		fmt.Fprint(w, `{"choices":[{"message":{"content":"late"}}]}`)
	})
	t.Setenv("LLM_TIMEOUT_SECONDS", "1")

	client := NewClient()
	_, err := client.Complete(context.Background(), "s", "u", 0.5)
	if err == nil {
		t.Fatal("Complete succeeded, want timeout error")
	}
	if !strings.Contains(err.Error(), "ollama api call") {
		t.Errorf("error %q is not wrapped as an ollama api call failure", err)
	}
}

func TestCompleteContextCancellation(t *testing.T) {
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"late"}}]}`)
	})

	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Complete(ctx, "s", "u", 0.5)
	if err == nil {
		t.Fatal("Complete succeeded, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v does not wrap context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "ollama api call") {
		t.Errorf("error %q is not wrapped as an ollama api call failure", err)
	}
}

// unsetEnv removes the variable for the duration of the test and
// restores (or keeps unset) its previous value afterwards, since
// t.Setenv cannot represent "variable not defined".
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, defined := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if defined {
			os.Setenv(key, orig)
		}
	})
}

func TestNewClientDefaults(t *testing.T) {
	// Defaults are exercised without OLLAMA_* overrides.
	unsetEnv(t, "OLLAMA_BASE_URL")
	unsetEnv(t, "OLLAMA_MODEL")
	unsetEnv(t, "LLM_TIMEOUT_SECONDS")
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want default %q", client.baseURL, DefaultBaseURL)
	}
	if client.model != DefaultModel {
		t.Errorf("model = %q, want default %q", client.model, DefaultModel)
	}
	if client.httpClient == nil || client.httpClient.Timeout != defaultTimeout {
		t.Errorf("unexpected http client: %+v", client.httpClient)
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:9999/")
	client := NewClient()
	if client.baseURL != "http://127.0.0.1:9999" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", client.baseURL)
	}
}

// TestCompleteErrorDoesNotEchoUpstreamBody guards against leaking provider
// response bodies into errors that handlers return to HTTP clients.
func TestCompleteErrorDoesNotEchoUpstreamBody(t *testing.T) {
	const secretDetail = "internal-request-detail-XYZ"
	chatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"`+secretDetail+`"}`)
	})
	client := NewClient()
	_, err := client.Complete(context.Background(), "s", "u", 0.7)
	if err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
	if strings.Contains(err.Error(), secretDetail) {
		t.Fatalf("error leaks upstream body: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should keep the status code: %v", err)
	}
}
