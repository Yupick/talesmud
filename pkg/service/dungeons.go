package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities/dungeons"
	r "github.com/talesmud/talesmud/pkg/repository"
)

// DungeonListFilter narrows DungeonsService.List queries. Zero values mean
// "no filter"; Limit <= 0 means "no limit".
type DungeonListFilter struct {
	Name   string
	Author string
	Limit  int
}

// DungeonsService provides logical functions on top of the dungeons
// repository (FASE 5.1 dungeon builder). The Definition payload stays an
// opaque json.RawMessage: only the frontend builder owns its shape.
type DungeonsService interface {
	List(filter DungeonListFilter) ([]*dungeons.DungeonDefinition, error)
	FindByID(id string) (*dungeons.DungeonDefinition, error)
	Create(def *dungeons.DungeonDefinition) (*dungeons.DungeonDefinition, error)
	// Update replaces the stored definition under id (404-style error when the
	// id is unknown) and preserves the original CreatedAt stamp.
	Update(id string, def *dungeons.DungeonDefinition) error
	Delete(id string) error
}

type dungeonsService struct {
	repo r.DungeonsRepository
}

// NewDungeonsService creates a new dungeons service.
func NewDungeonsService(dungeonsRepo r.DungeonsRepository) DungeonsService {
	return &dungeonsService{repo: dungeonsRepo}
}

// ValidateDungeonDefinition enforces the invariants shared by create and
// update: a non-empty name, a grid larger than zero on both axes, and an
// opaque definition payload that parses as non-empty JSON. The payload
// content is not interpreted.
func ValidateDungeonDefinition(def *dungeons.DungeonDefinition) error {
	if def == nil {
		return errors.New("dungeon definition is required")
	}
	if strings.TrimSpace(def.Name) == "" {
		return errors.New("dungeon name is required")
	}
	if def.GridWidth <= 0 || def.GridHeight <= 0 {
		return errors.New("gridWidth and gridHeight must be greater than zero")
	}
	trimmed := bytes.TrimSpace(def.Definition)
	if len(trimmed) == 0 {
		return errors.New("definition is required")
	}
	if !json.Valid(trimmed) {
		return errors.New("definition must be valid JSON")
	}
	return nil
}

func (s *dungeonsService) List(filter DungeonListFilter) ([]*dungeons.DungeonDefinition, error) {
	name := strings.TrimSpace(filter.Name)
	author := strings.TrimSpace(filter.Author)

	var (
		list []*dungeons.DungeonDefinition
		err  error
	)
	switch {
	case name != "":
		list, err = s.repo.FindByName(name)
	case author != "":
		list, err = s.repo.FindByAuthor(author)
	default:
		list, err = s.repo.FindAll()
	}
	if err != nil {
		return nil, err
	}

	// FindByName/FindByAuthor match exactly on one field each; when both
	// filters are set, the second one is narrowed in memory.
	if name != "" && author != "" {
		filtered := make([]*dungeons.DungeonDefinition, 0, len(list))
		for _, def := range list {
			if def != nil && strings.TrimSpace(def.Author) == author {
				filtered = append(filtered, def)
			}
		}
		list = filtered
	}

	if filter.Limit > 0 && len(list) > filter.Limit {
		list = list[:filter.Limit]
	}
	return list, nil
}

func (s *dungeonsService) FindByID(id string) (*dungeons.DungeonDefinition, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("dungeon id is required")
	}
	return s.repo.FindByID(id)
}

func (s *dungeonsService) Create(def *dungeons.DungeonDefinition) (*dungeons.DungeonDefinition, error) {
	if err := ValidateDungeonDefinition(def); err != nil {
		return nil, err
	}
	def.Name = strings.TrimSpace(def.Name)
	def.Author = strings.TrimSpace(def.Author)
	// Never trust a client-supplied id on create.
	def.Entity = nil
	return s.repo.Create(def)
}

func (s *dungeonsService) Update(id string, def *dungeons.DungeonDefinition) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("dungeon id is required")
	}
	if err := ValidateDungeonDefinition(def); err != nil {
		return err
	}
	def.Name = strings.TrimSpace(def.Name)
	def.Author = strings.TrimSpace(def.Author)

	// Fetch first so an unknown id fails with the repo's not-found error and
	// the original CreatedAt survives the full replace.
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	def.CreatedAt = existing.CreatedAt
	return s.repo.Update(id, def)
}

func (s *dungeonsService) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("dungeon id is required")
	}
	return s.repo.Delete(id)
}
