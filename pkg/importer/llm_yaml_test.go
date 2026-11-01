package importer_test

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/importer"
	"gopkg.in/yaml.v3"
)

func TestYAMLNPCParsesCamelCaseLLMConfig(t *testing.T) {
	raw := []byte(`id: NPC_LLM
name: Crypt Keeper
level: 2
maxHitPoints: 40
spawnRoomId: R1001
llm:
  enabled: true
  persona: "Soy el guardián de la cripta."
  voice: "grave"
  temperature: 0.7
  memoryTurns: 8
  fallbackDialogId: DLG_CRYPT
  cooldownSeconds: 5
  personality:
    openness: 0.4
    conscientiousness: 0.9
    extraversion: 0.6
    agreeableness: 0.5
    neuroticism: 0.3
  goals:
    - "Proteger la cripta."
  secrets:
    - "El rey duerme bajo el altar."
  relations:
    - target: player
      kind: mentor
      sentiment: 0.7
`)

	var source importer.YAMLNPC
	if err := yaml.Unmarshal(raw, &source); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if source.LLM == nil || !source.LLM.Enabled {
		t.Fatalf("llm.enabled not parsed: %+v", source.LLM)
	}
	if source.LLM.MemoryTurns != 8 || source.LLM.CooldownSeconds != 5 ||
		source.LLM.FallbackDialogID != "DLG_CRYPT" || source.LLM.Temperature != 0.7 {
		t.Fatalf("camelCase scalar fields not parsed: %+v", source.LLM)
	}
	if source.LLM.Personality == nil || source.LLM.Personality.Openness != 0.4 ||
		source.LLM.Personality.Conscientiousness != 0.9 {
		t.Fatalf("personality not parsed: %+v", source.LLM.Personality)
	}
	if len(source.LLM.Goals) != 1 || len(source.LLM.Secrets) != 1 {
		t.Fatalf("goals/secrets not parsed: %+v", source.LLM)
	}
	if len(source.LLM.Relations) != 1 || source.LLM.Relations[0].Target != "player" ||
		source.LLM.Relations[0].Kind != "mentor" || source.LLM.Relations[0].Sentiment != 0.7 {
		t.Fatalf("relations not parsed: %+v", source.LLM.Relations)
	}

	n := source.ToEntity()
	if n.LLM == nil || n.LLM.MemoryTurns != 8 || n.LLM.Personality == nil ||
		n.LLM.Personality.Extraversion != 0.6 {
		t.Fatalf("llm not converted to entity: %+v", n.LLM)
	}

	// ToEntity must deep-copy the LLM config.
	n.LLM.Goals[0] = "mutated"
	if source.LLM.Goals[0] != "Proteger la cripta." {
		t.Fatalf("entity shares goals with YAML source: %+v", source.LLM.Goals)
	}
}
