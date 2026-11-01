package llmgen

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// seededFallback picks a stable pseudo-random value for v so that the same
// request yields the same deterministic output, but different requests vary.
func seededFallback(seed uint64, values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[seed%uint64(len(values))]
}

func seedOf(req Request) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(string(req.Task)))
	keys := sortedVarKeys(req.Vars)
	for _, k := range keys {
		if strings.HasPrefix(k, "_") {
			continue
		}
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(fmt.Sprintf("%v", req.Vars[k])))
	}
	return h.Sum64()
}

func sortedVarKeys(vars map[string]any) []string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func stringVar(vars map[string]any, key string) string {
	if v, ok := vars[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

// builtinFallback produces deterministic content for each built-in task.
func builtinFallback(req Request, rendered llmprompt.Rendered) (Response, error) {
	seed := seedOf(req)
	var text string
	switch req.Task {
	case TaskCharacterName:
		text = deterministicName(seed, req.Vars)
	case TaskCharacterDescription:
		text = deterministicDescription(seed, req.Vars)
	case TaskRoomDescription:
		text = deterministicRoom(req.Vars)
	case TaskDMNarration:
		// Design §6.5: when no LLM is available the Dungeon Master stays
		// silent. A deterministic line would read as authored content and
		// could leak stale combat facts.
		return Response{Task: req.Task, Source: SourceDeterministic}, nil
	case TaskQuestGenerate:
		payload, err := deterministicQuestJSON(req, rendered)
		if err != nil {
			return Response{}, err
		}
		return Response{
			Task:   req.Task,
			Text:   string(payload),
			JSON:   payload,
			Source: SourceDeterministic,
		}, nil
	default:
		return Response{}, fmt.Errorf("llmgen: no deterministic fallback for task %q", req.Task)
	}
	return Response{
		Task:   req.Task,
		Text:   text,
		JSON:   json.RawMessage(text),
		Source: SourceDeterministic,
	}, nil
}

var (
	deterministicSyllables = []string{
		"ar", "bran", "dor", "el", "fen", "gal", "hal", "is", "kel", "lor",
		"mor", "ny", "or", "ra", "sil", "tar", "ul", "ven", "wyn", "zar",
		"th", "dr", "ka", "mi", "no", "va", "ce", "lu", "grim", "ash",
	}
	deterministicTraits = []string{
		"con mirada cansada pero alerta",
		"de porte tranquilo y modales medidos",
		"cubierto de polvo de caminos lejanos",
		"de sonrisa escasa y voz grave",
		"con cicatrices que no explica",
		"de andar silencioso y atenta curiosidad",
	}
)

func deterministicName(seed uint64, vars map[string]any) string {
	parts := 2 + int(seed%2)
	var b strings.Builder
	for i := 0; i < parts; i++ {
		idx := (seed >> (8 * uint(i))) % uint64(len(deterministicSyllables))
		b.WriteString(deterministicSyllables[idx])
	}
	name := b.String()
	if name == "" {
		return "Aldeano"
	}
	runes := []rune(name)
	name = strings.ToUpper(string(runes[0])) + string(runes[1:])
	return llmprompt.Sanitize(name, 80)
}

func deterministicDescription(seed uint64, vars map[string]any) string {
	race := stringVar(vars, "race")
	if race == "" {
		race = "viajero"
	}
	origin := stringVar(vars, "originArea")
	origin = llmprompt.Sanitize(origin, 80)
	trait := seededFallback(seed, deterministicTraits)
	if origin != "" {
		return fmt.Sprintf("Un %s de %s, %s.", race, origin, trait)
	}
	return fmt.Sprintf("Un %s, %s.", race, trait)
}

func deterministicRoom(vars map[string]any) string {
	current := llmprompt.Sanitize(stringVar(vars, "currentDescription"), 300)
	if current != "" {
		return current
	}
	name := llmprompt.Sanitize(stringVar(vars, "name"), 80)
	if name == "" {
		name = "la estancia"
	}
	return fmt.Sprintf("Te encuentras en %s. El lugar guarda un silencio expectante.", name)
}
