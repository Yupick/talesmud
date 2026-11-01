package llm

import (
	"os"
	"testing"

	"github.com/talesmud/talesmud/pkg/service/groq"
	"github.com/talesmud/talesmud/pkg/service/ollama"
)

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

func TestNewFromEnvOllamaProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "ollama")
	t.Setenv("GROQ_API_KEY", "")

	client := NewFromEnv()
	if client == nil {
		t.Fatal("NewFromEnv returned nil for LLM_PROVIDER=ollama")
	}
	if _, ok := client.(*ollama.Client); !ok {
		t.Fatalf("client type = %T, want *ollama.Client", client)
	}
}

func TestNewFromEnvGroqWithoutKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "groq")
	t.Setenv("GROQ_API_KEY", "")

	if client := NewFromEnv(); client != nil {
		t.Fatalf("NewFromEnv returned %T for LLM_PROVIDER=groq without GROQ_API_KEY, want nil", client)
	}
}

func TestNewFromEnvGroqWithKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "groq")
	t.Setenv("GROQ_API_KEY", "test-key")

	client := NewFromEnv()
	if client == nil {
		t.Fatal("NewFromEnv returned nil for LLM_PROVIDER=groq with GROQ_API_KEY")
	}
	if _, ok := client.(*groq.Client); !ok {
		t.Fatalf("client type = %T, want *groq.Client", client)
	}
}

func TestNewFromEnvDisabled(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "none")
	t.Setenv("GROQ_API_KEY", "test-key") // must be ignored

	if client := NewFromEnv(); client != nil {
		t.Fatalf("NewFromEnv returned %T for LLM_PROVIDER=none, want nil", client)
	}
}

func TestNewFromEnvOffAndEmpty(t *testing.T) {
	for _, value := range []string{"off", ""} {
		t.Setenv("LLM_PROVIDER", value)
		t.Setenv("GROQ_API_KEY", "test-key")
		if client := NewFromEnv(); client != nil {
			t.Fatalf("NewFromEnv returned %T for LLM_PROVIDER=%q, want nil", client, value)
		}
	}
}

func TestNewFromEnvUnsetWithGroqKey(t *testing.T) {
	unsetEnv(t, "LLM_PROVIDER")
	t.Setenv("GROQ_API_KEY", "test-key")

	client := NewFromEnv()
	if client == nil {
		t.Fatal("NewFromEnv returned nil for unset LLM_PROVIDER with GROQ_API_KEY")
	}
	if _, ok := client.(*groq.Client); !ok {
		t.Fatalf("client type = %T, want *groq.Client (legacy behavior)", client)
	}
}

func TestNewFromEnvUnsetWithoutGroqKey(t *testing.T) {
	unsetEnv(t, "LLM_PROVIDER")
	unsetEnv(t, "GROQ_API_KEY")

	client := NewFromEnv()
	if client == nil {
		t.Fatal("NewFromEnv returned nil for unset LLM_PROVIDER without GROQ_API_KEY")
	}
	if _, ok := client.(*ollama.Client); !ok {
		t.Fatalf("client type = %T, want *ollama.Client (local default)", client)
	}
}

func TestNewFromEnvUnknownProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "unknown-provider")
	t.Setenv("GROQ_API_KEY", "test-key")

	if client := NewFromEnv(); client != nil {
		t.Fatalf("NewFromEnv returned %T for unknown LLM_PROVIDER, want nil", client)
	}
}

func TestNewFromEnvProviderIsCaseInsensitive(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "  Ollama ")
	t.Setenv("GROQ_API_KEY", "")

	client := NewFromEnv()
	if client == nil {
		t.Fatal("NewFromEnv returned nil for padded uppercase LLM_PROVIDER")
	}
	if _, ok := client.(*ollama.Client); !ok {
		t.Fatalf("client type = %T, want *ollama.Client", client)
	}
}
