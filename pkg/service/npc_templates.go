package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	r "github.com/talesmud/talesmud/pkg/repository"
)

// NPCTemplateListFilter narrows NPCTemplatesService.List queries. Zero values
// mean "no filter"; Limit <= 0 means "no limit".
type NPCTemplateListFilter struct {
	Name  string
	Role  string
	Limit int
}

// NPCTemplatesService provides logical functions on top of the NPC template
// repository (FASE 5.3). Loot, Behavior and Scaling stay opaque json.RawMessage
// payloads: only the frontend authoring tools own their shape.
type NPCTemplatesService interface {
	List(filter NPCTemplateListFilter) ([]*npc.NPCTemplate, error)
	FindByID(id string) (*npc.NPCTemplate, error)
	Create(tpl *npc.NPCTemplate) (*npc.NPCTemplate, error)
	// Update replaces the stored template under id (not-found error when the id
	// is unknown) and preserves the original CreatedAt stamp.
	Update(id string, tpl *npc.NPCTemplate) error
	Delete(id string) error
}

type npcTemplatesService struct {
	repo r.NPCTemplatesRepository
}

// NewNPCTemplatesService creates a new NPC template service.
func NewNPCTemplatesService(repo r.NPCTemplatesRepository) NPCTemplatesService {
	return &npcTemplatesService{repo: repo}
}

// ValidateNPCTemplate enforces the invariants shared by create and update:
// non-empty name, level/maxHp at least one, non-negative damage bounded by
// damageMax, goldMax >= goldMin, and any supplied loot/behavior/scaling payload
// parsing as valid JSON. The payload content itself is not interpreted.
func ValidateNPCTemplate(tpl *npc.NPCTemplate) error {
	if tpl == nil {
		return errors.New("npc template is required")
	}
	if strings.TrimSpace(tpl.Name) == "" {
		return errors.New("npc template name is required")
	}
	if tpl.Level < 1 {
		return errors.New("level must be greater than or equal to one")
	}
	if tpl.MaxHp < 1 {
		return errors.New("maxHp must be greater than or equal to one")
	}
	if tpl.DamageMin < 0 || tpl.DamageMax < 0 {
		return errors.New("damageMin and damageMax must be greater than or equal to zero")
	}
	if tpl.DamageMax < tpl.DamageMin {
		return errors.New("damageMax must be greater than or equal to damageMin")
	}
	if tpl.GoldMax < tpl.GoldMin {
		return errors.New("goldMax must be greater than or equal to goldMin")
	}
	for _, field := range []struct {
		name string
		raw  json.RawMessage
	}{
		{"loot", tpl.Loot},
		{"behavior", tpl.Behavior},
		{"scaling", tpl.Scaling},
	} {
		trimmed := bytes.TrimSpace(field.raw)
		if len(trimmed) == 0 {
			continue
		}
		if !json.Valid(trimmed) {
			return errors.New(field.name + " must be valid JSON")
		}
	}
	return nil
}

func (s *npcTemplatesService) List(filter NPCTemplateListFilter) ([]*npc.NPCTemplate, error) {
	name := strings.TrimSpace(filter.Name)
	role := strings.TrimSpace(filter.Role)

	var (
		list []*npc.NPCTemplate
		err  error
	)
	switch {
	case name != "":
		list, err = s.repo.FindByName(name)
	case role != "":
		list, err = s.repo.FindByRole(role)
	default:
		list, err = s.repo.FindAll()
	}
	if err != nil {
		return nil, err
	}

	// FindByName/FindByRole match exactly on one field each; when both filters
	// are set, the second one is narrowed in memory.
	if name != "" && role != "" {
		filtered := make([]*npc.NPCTemplate, 0, len(list))
		for _, tpl := range list {
			if tpl != nil && strings.TrimSpace(tpl.Role) == role {
				filtered = append(filtered, tpl)
			}
		}
		list = filtered
	}

	if filter.Limit > 0 && len(list) > filter.Limit {
		list = list[:filter.Limit]
	}
	return list, nil
}

func (s *npcTemplatesService) FindByID(id string) (*npc.NPCTemplate, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("npc template id is required")
	}
	return s.repo.FindByID(id)
}

func (s *npcTemplatesService) Create(tpl *npc.NPCTemplate) (*npc.NPCTemplate, error) {
	if err := ValidateNPCTemplate(tpl); err != nil {
		return nil, err
	}
	tpl.Name = strings.TrimSpace(tpl.Name)
	tpl.Role = strings.TrimSpace(tpl.Role)
	tpl.CreatureType = strings.TrimSpace(tpl.CreatureType)
	// Templates authored through this API are always flagged as such.
	tpl.IsTemplate = true
	// Never trust a client-supplied id on create.
	tpl.Entity = nil
	return s.repo.Create(tpl)
}

func (s *npcTemplatesService) Update(id string, tpl *npc.NPCTemplate) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("npc template id is required")
	}
	if err := ValidateNPCTemplate(tpl); err != nil {
		return err
	}
	tpl.Name = strings.TrimSpace(tpl.Name)
	tpl.Role = strings.TrimSpace(tpl.Role)
	tpl.CreatureType = strings.TrimSpace(tpl.CreatureType)
	tpl.IsTemplate = true

	// Fetch first so an unknown id fails with the repo's not-found error and
	// the original CreatedAt survives the full replace.
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	tpl.CreatedAt = existing.CreatedAt
	return s.repo.Update(id, tpl)
}

func (s *npcTemplatesService) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("npc template id is required")
	}
	return s.repo.Delete(id)
}
