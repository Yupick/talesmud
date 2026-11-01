package npc

import "math"

// LLMPersonality captures Big5-like traits used to steer LLM tone.
// All values are expected in [0,1] and are additive/optional for
// backwards compatibility.
type LLMPersonality struct {
	Openness          float64 `bson:"openness,omitempty" json:"openness,omitempty" yaml:"openness,omitempty"` // 0..1
	Conscientiousness float64 `bson:"conscientiousness,omitempty" json:"conscientiousness,omitempty" yaml:"conscientiousness,omitempty"`
	Extraversion      float64 `bson:"extraversion,omitempty" json:"extraversion,omitempty" yaml:"extraversion,omitempty"`
	Agreeableness     float64 `bson:"agreeableness,omitempty" json:"agreeableness,omitempty" yaml:"agreeableness,omitempty"`
	Neuroticism       float64 `bson:"neuroticism,omitempty" json:"neuroticism,omitempty" yaml:"neuroticism,omitempty"`
}

// LLMRelation captures a directed/targeted attitude the NPC holds.
// Target can be an NPC name/id, a faction or "player". Kind is one of
// ally|rival|family|faction|mentor. Sentiment in [-1,1].
type LLMRelation struct {
	Target    string  `bson:"target,omitempty" json:"target,omitempty" yaml:"target,omitempty"`          // nombre/ID de NPC, facción o "player"
	Kind      string  `bson:"kind,omitempty" json:"kind,omitempty" yaml:"kind,omitempty"`                // ally|rival|family|faction|mentor
	Sentiment float64 `bson:"sentiment,omitempty" json:"sentiment,omitempty" yaml:"sentiment,omitempty"` // -1..1
}

// LLMConfig is the per-NPC opt-in for free-form, model-driven dialogue
// (docs/design/PHASE6_LLM.md §6.3). The NPC table is (id, data JSON), so a new
// field needs no migration; legacy rows simply decode with LLM == nil.
type LLMConfig struct {
	Enabled          bool            `bson:"enabled,omitempty" json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Persona          string          `bson:"persona,omitempty" json:"persona,omitempty" yaml:"persona,omitempty"`                            // 1-3 sentences, first person
	Voice            string          `bson:"voice,omitempty" json:"voice,omitempty" yaml:"voice,omitempty"`                                  // register: "brusco", "anciano"
	Temperature      float64         `bson:"temperature,omitempty" json:"temperature,omitempty" yaml:"temperature,omitempty"`                // 0 -> prompt default (0.8)
	MemoryTurns      int             `bson:"memoryTurns,omitempty" json:"memoryTurns,omitempty" yaml:"memoryTurns,omitempty"`                // 0 -> 8
	FallbackDialogID string          `bson:"fallbackDialogId,omitempty" json:"fallbackDialogId,omitempty" yaml:"fallbackDialogId,omitempty"` // authored dialog to fall back to
	CooldownSeconds  int             `bson:"cooldownSeconds,omitempty" json:"cooldownSeconds,omitempty" yaml:"cooldownSeconds,omitempty"`    // 0 -> 3
	Personality      *LLMPersonality `bson:"personality,omitempty" json:"personality,omitempty" yaml:"personality,omitempty"`
	Goals            []string        `bson:"goals,omitempty" json:"goals,omitempty" yaml:"goals,omitempty"`
	Secrets          []string        `bson:"secrets,omitempty" json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Relations        []LLMRelation   `bson:"relations,omitempty" json:"relations,omitempty" yaml:"relations,omitempty"`
}

// LLM defaults; mirrors the rules in docs/design/PHASE6_LLM.md §6.3.
const (
	DefaultLLMMemoryTurns     = 8
	DefaultLLMCooldownSeconds = 3

	// Relation sentiment bounds and the maximum delta one interaction may
	// apply. They are shared by the game runtime and the memory service so a
	// single NPC/player exchange can never move a relationship more than 0.05
	// or push it outside [-1,1].
	MinRelationSentiment = -1.0
	MaxRelationSentiment = 1.0
	MaxRelationDelta     = 0.05
)

// ClampRelationSentiment bounds a relationship sentiment to [-1,1].
func ClampRelationSentiment(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < MinRelationSentiment {
		return MinRelationSentiment
	}
	if v > MaxRelationSentiment {
		return MaxRelationSentiment
	}
	return v
}

// ClampRelationDelta bounds one interaction delta to [-0.05,+0.05].
func ClampRelationDelta(delta float64) float64 {
	if math.IsNaN(delta) {
		return 0
	}
	if delta < -MaxRelationDelta {
		return -MaxRelationDelta
	}
	if delta > MaxRelationDelta {
		return MaxRelationDelta
	}
	return delta
}

// ApplyRelationDelta adds a bounded delta to a relationship and clamps the
// result to [-1,1]. It is the only mutation primitive used by the runtime.
func ApplyRelationDelta(current, delta float64) float64 {
	return ClampRelationSentiment(ClampRelationSentiment(current) + ClampRelationDelta(delta))
}

// LLMEnabled reports whether this NPC opts into LLM dialogue. Nil-safe so
// legacy NPCs (LLM == nil) short-circuit without touching the field.
func (npc *NPC) LLMEnabled() bool {
	return npc != nil && npc.LLM != nil && npc.LLM.Enabled
}

// EffectiveMemoryTurns returns the short-term window size, defaulting when the
// config left it unset.
func (npc *NPC) EffectiveMemoryTurns() int {
	if npc == nil || npc.LLM == nil || npc.LLM.MemoryTurns <= 0 {
		return DefaultLLMMemoryTurns
	}
	return npc.LLM.MemoryTurns
}

// EffectiveCooldownSeconds returns the per-NPC cooldown, defaulting when unset.
func (npc *NPC) EffectiveCooldownSeconds() int {
	if npc == nil || npc.LLM == nil || npc.LLM.CooldownSeconds <= 0 {
		return DefaultLLMCooldownSeconds
	}
	return npc.LLM.CooldownSeconds
}

// EffectivePersonality returns the personality config if present, else nil.
// Nil-safe helper for prompt construction.
func (c *LLMConfig) EffectivePersonality() *LLMPersonality {
	if c == nil {
		return nil
	}
	return c.Personality
}

// HasPersonality reports whether a personality is configured.
func (c *LLMConfig) HasPersonality() bool {
	if c == nil || c.Personality == nil {
		return false
	}
	p := c.Personality
	return p.Openness != 0 || p.Conscientiousness != 0 || p.Extraversion != 0 || p.Agreeableness != 0 || p.Neuroticism != 0
}

// Clone returns a deep copy of the LLMConfig to avoid aliasing.
func (c *LLMConfig) Clone() *LLMConfig {
	if c == nil {
		return nil
	}
	out := *c
	if c.Personality != nil {
		p := *c.Personality
		out.Personality = &p
	}
	if len(c.Goals) > 0 {
		out.Goals = append([]string(nil), c.Goals...)
	}
	if len(c.Secrets) > 0 {
		out.Secrets = append([]string(nil), c.Secrets...)
	}
	if len(c.Relations) > 0 {
		out.Relations = make([]LLMRelation, len(c.Relations))
		copy(out.Relations, c.Relations)
	}
	return &out
}
