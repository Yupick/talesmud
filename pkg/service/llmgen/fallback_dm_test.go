package llmgen

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// TestDMNarrationFallbackIsSilent pins design §6.5: without an LLM the DM
// emits nothing. A deterministic line would be indistinguishable from
// authored prose and could carry stale combat facts.
func TestDMNarrationFallbackIsSilent(t *testing.T) {
	resp, err := builtinFallback(Request{
		Task: TaskDMNarration,
		Vars: map[string]any{
			"event":   "combatEnd",
			"outcome": "victory",
			"round":   3,
			"party":   2,
		},
	}, llmprompt.Rendered{})
	if err != nil {
		t.Fatalf("builtinFallback: %v", err)
	}
	if resp.Text != "" {
		t.Errorf("DM fallback text = %q, want silence", resp.Text)
	}
	if resp.Source != SourceDeterministic {
		t.Errorf("source = %q, want deterministic", resp.Source)
	}
}
