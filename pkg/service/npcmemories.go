package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/repository"
)

// Long-term NPC memory defaults. They mirror docs/design/PHASE6_LLM.md §6.3:
// a bounded rolling window of dialogue turns, a mergeable fact list and a
// plain concatenated summary once the window fills.
const (
	DefaultNPCMemoryTurns    = 8
	DefaultNPCMemoryMaxTurns = 50
	DefaultNPCMemoryMaxFacts = 24

	npcmemoryLastLineRunes = 200
	npcmemoryFactRunes     = 200
	npcmemorySummaryRunes  = 400
)

// NPCMemoryConfig tunes the NPCMemoriesService window. Zero values fall back
// to the package defaults.
type NPCMemoryConfig struct {
	// SummaryAtTurns is the turn count that triggers a summary rebuild.
	SummaryAtTurns int
	// MaxTurns caps the stored turn counter so it cannot grow without bound.
	MaxTurns int
	// MaxFacts caps the stored fact list; the most recent facts are kept.
	MaxFacts int
	// MaxSummaryRunes caps the generated summary length.
	MaxSummaryRunes int
}

func (c NPCMemoryConfig) withDefaults() NPCMemoryConfig {
	if c.SummaryAtTurns <= 0 {
		c.SummaryAtTurns = DefaultNPCMemoryTurns
	}
	if c.MaxTurns <= 0 {
		c.MaxTurns = DefaultNPCMemoryMaxTurns
	}
	if c.MaxFacts <= 0 {
		c.MaxFacts = DefaultNPCMemoryMaxFacts
	}
	if c.MaxSummaryRunes <= 0 {
		c.MaxSummaryRunes = npcmemorySummaryRunes
	}
	if c.SummaryAtTurns > c.MaxTurns {
		c.SummaryAtTurns = c.MaxTurns
	}
	return c
}

// NPCMemoriesService is the business layer over the npc_memories repository.
// It owns the JSON entity shape, so callers deal in repository.NPCMemory.
type NPCMemoriesService interface {
	// Get returns the memory of npcID about characterID, or (nil, nil) when
	// the NPC has no memory of that character yet.
	Get(npcID, characterID string) (*repository.NPCMemory, error)
	// Save (alias of Upsert) creates or replaces one memory row.
	Save(m *repository.NPCMemory) (*repository.NPCMemory, error)
	// Upsert creates or replaces one memory row.
	Upsert(m *repository.NPCMemory) (*repository.NPCMemory, error)
	// Delete forgets one (npc, character) pair.
	Delete(npcID, characterID string) error
	// DeleteForNPC forgets every character a single NPC remembers.
	DeleteForNPC(npcID string) error
	// RecordTurn merges one dialogue turn into the stored memory: it bumps the
	// turn counter, appends/dedupes facts, stores the last user line and
	// rebuilds the summary once the window threshold is crossed.
	RecordTurn(npcID, characterID, lastUserLine string, facts []string) (*repository.NPCMemory, error)
	// AdjustRelation applies one bounded sentiment delta ([-0.05,+0.05]) to a
	// target (normally "player") and clamps the stored value to [-1,1]. It only
	// touches the memory row, never HP/XP/quests/inventory.
	AdjustRelation(npcID, characterID, target string, delta float64) (*repository.NPCMemory, error)
	// SummaryAtTurns exposes the threshold used to trigger summary rebuilds.
	SummaryAtTurns() int
}

type npcMemoriesService struct {
	repo repository.NPCMemoriesRepository
	cfg  NPCMemoryConfig
}

// NewNPCMemoriesService builds the service with the package defaults.
func NewNPCMemoriesService(repo repository.NPCMemoriesRepository) NPCMemoriesService {
	return NewNPCMemoriesServiceWithConfig(repo, NPCMemoryConfig{})
}

// NewNPCMemoriesServiceWithConfig builds the service with explicit tuning.
func NewNPCMemoriesServiceWithConfig(repo repository.NPCMemoriesRepository, cfg NPCMemoryConfig) NPCMemoriesService {
	return &npcMemoriesService{repo: repo, cfg: cfg.withDefaults()}
}

func (s *npcMemoriesService) Get(npcID, characterID string) (*repository.NPCMemory, error) {
	if strings.TrimSpace(npcID) == "" || strings.TrimSpace(characterID) == "" {
		return nil, errors.New("npcId and characterId are required")
	}
	if s.repo == nil {
		return nil, errors.New("npc memories repository is not configured")
	}
	return s.repo.Get(npcID, characterID)
}

func (s *npcMemoriesService) Save(m *repository.NPCMemory) (*repository.NPCMemory, error) {
	return s.Upsert(m)
}

func (s *npcMemoriesService) Upsert(m *repository.NPCMemory) (*repository.NPCMemory, error) {
	if m == nil {
		return nil, errors.New("memory is nil")
	}
	if strings.TrimSpace(m.NPCID) == "" || strings.TrimSpace(m.CharacterID) == "" {
		return nil, errors.New("memory requires npcId and characterId")
	}
	if s.repo == nil {
		return nil, errors.New("npc memories repository is not configured")
	}
	m.Summary = sanitizeMemoryLine(m.Summary, s.cfg.MaxSummaryRunes)
	m.Facts = mergeMemoryFacts(nil, m.Facts, s.cfg.MaxFacts)
	m.Relations = normalizeMemoryRelations(m.Relations)
	if m.Turns > s.cfg.MaxTurns {
		m.Turns = s.cfg.MaxTurns
	}
	return s.repo.Upsert(m)
}

func (s *npcMemoriesService) Delete(npcID, characterID string) error {
	if s.repo == nil {
		return errors.New("npc memories repository is not configured")
	}
	return s.repo.Delete(npcID, characterID)
}

func (s *npcMemoriesService) DeleteForNPC(npcID string) error {
	if strings.TrimSpace(npcID) == "" {
		return errors.New("npcId is required")
	}
	if s.repo == nil {
		return errors.New("npc memories repository is not configured")
	}
	return s.repo.DeleteForNPC(npcID)
}

func (s *npcMemoriesService) SummaryAtTurns() int {
	return s.cfg.SummaryAtTurns
}

func (s *npcMemoriesService) AdjustRelation(npcID, characterID, target string, delta float64) (*repository.NPCMemory, error) {
	if strings.TrimSpace(npcID) == "" || strings.TrimSpace(characterID) == "" {
		return nil, errors.New("npcId and characterId are required")
	}
	if s.repo == nil {
		return nil, errors.New("npc memories repository is not configured")
	}
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "player"
	}
	if npc.ClampRelationDelta(delta) == 0 {
		return s.repo.Get(npcID, characterID)
	}

	m, err := s.repo.Get(npcID, characterID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &repository.NPCMemory{NPCID: npcID, CharacterID: characterID}
	}
	if m.Relations == nil {
		m.Relations = make(map[string]float64)
	}
	m.Relations[target] = npc.ApplyRelationDelta(m.Relations[target], delta)
	m.Relations = normalizeMemoryRelations(m.Relations)
	m.Updated = time.Now()
	return s.repo.Upsert(m)
}

func (s *npcMemoriesService) RecordTurn(npcID, characterID, lastUserLine string, facts []string) (*repository.NPCMemory, error) {
	if strings.TrimSpace(npcID) == "" || strings.TrimSpace(characterID) == "" {
		return nil, errors.New("npcId and characterId are required")
	}
	if s.repo == nil {
		return nil, errors.New("npc memories repository is not configured")
	}

	m, err := s.repo.Get(npcID, characterID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &repository.NPCMemory{NPCID: npcID, CharacterID: characterID}
	}

	// The async LLM summariser writes Summary with the "llm" marker. Preserve
	// it instead of rebuilding a deterministic summary every threshold turn.
	// Legacy rows have no marker: a summary that differs from the pre-turn
	// deterministic rebuild came from the model (or an author) and is preserved
	// too, while the old deterministic summaries keep refreshing as before.
	previousDeterministic := buildMemorySummary(m, s.cfg.MaxSummaryRunes)
	preserveSummary := m.SummarySource == repository.NPCMemorySummaryLLM ||
		(strings.TrimSpace(m.Summary) != "" && m.Summary != previousDeterministic)

	if line := sanitizeMemoryLine(lastUserLine, npcmemoryLastLineRunes); line != "" {
		m.LastUserLine = line
	}
	m.Facts = mergeMemoryFacts(m.Facts, facts, s.cfg.MaxFacts)

	m.Turns++
	if m.Turns > s.cfg.MaxTurns {
		m.Turns = s.cfg.MaxTurns
	}
	if m.Turns >= s.cfg.SummaryAtTurns && !preserveSummary {
		m.Summary = buildMemorySummary(m, s.cfg.MaxSummaryRunes)
		m.SummarySource = repository.NPCMemorySummaryDeterministic
	}
	m.Updated = time.Now()

	return s.repo.Upsert(m)
}

// normalizeMemoryRelations lower-cases targets and clamps every stored
// sentiment to [-1,1]. A nil input stays nil so legacy rows are untouched.
func normalizeMemoryRelations(in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for target, sentiment := range in {
		key := strings.ToLower(strings.TrimSpace(target))
		if key == "" {
			continue
		}
		out[key] = npc.ClampRelationSentiment(sentiment)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeMemoryFacts appends incoming facts to existing ones, dropping blanks
// and case-insensitive duplicates. When max > 0 the most recent facts win.
func mergeMemoryFacts(existing, incoming []string, max int) []string {
	out := make([]string, 0, len(existing)+len(incoming))
	seen := make(map[string]bool, len(existing)+len(incoming))
	add := func(raw string) {
		fact := sanitizeMemoryLine(raw, npcmemoryFactRunes)
		if fact == "" {
			return
		}
		key := strings.ToLower(fact)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, fact)
	}
	for _, f := range existing {
		add(f)
	}
	for _, f := range incoming {
		add(f)
	}
	if max > 0 && len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// buildMemorySummary concatenates the facts and the last user line. It is the
// simple concatenate/trim strategy the design allows until npc.memory_summary
// (LLM-based summarisation) lands in a later cut.
func buildMemorySummary(m *repository.NPCMemory, maxRunes int) string {
	if m == nil {
		return ""
	}
	parts := make([]string, 0, len(m.Facts)+1)
	for _, f := range m.Facts {
		if strings.TrimSpace(f) != "" {
			parts = append(parts, f)
		}
	}
	if line := strings.TrimSpace(m.LastUserLine); line != "" {
		parts = append(parts, "Última pregunta: "+line)
	}
	summary := strings.TrimSpace(strings.Join(parts, "; "))
	if maxRunes > 0 && utf8.RuneCountInString(summary) > maxRunes {
		summary = strings.TrimSpace(string([]rune(summary)[:maxRunes]))
	}
	return summary
}

// sanitizeMemoryLine collapses whitespace and caps one memory line at max runes.
func sanitizeMemoryLine(raw string, maxRunes int) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	s = strings.TrimSpace(s)
	if maxRunes > 0 && utf8.RuneCountInString(s) > maxRunes {
		s = strings.TrimSpace(string([]rune(s)[:maxRunes]))
	}
	return s
}
