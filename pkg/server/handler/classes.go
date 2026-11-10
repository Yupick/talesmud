package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities/characters"
)

// GetClasses returns the public class catalog. Balance numbers stay off this payload.
func GetClasses(c *gin.Context) {
	base := []classkit.RaceInfo{
		{ID: characters.RaceHuman.ID, Name: characters.RaceHuman.Name, Blurb: characters.RaceHuman.Description},
		{ID: characters.RaceDwarf.ID, Name: characters.RaceDwarf.Name, Blurb: characters.RaceDwarf.Description},
		{ID: characters.RaceElf.ID, Name: characters.RaceElf.Name, Blurb: characters.RaceElf.Description},
		{ID: characters.RaceConstruct.ID, Name: characters.RaceConstruct.Name, Blurb: characters.RaceConstruct.Description},
	}
	c.JSON(http.StatusOK, classkit.ClientView(base))
}

// ClassesHandler exposes the FASE 5.2 class-definition CRUD. Definitions are
// YAML files in the active world-pack directory, serialized through classkit.
type ClassesHandler struct{}

type classBalanceDTO struct {
	DamageDealt float64 `json:"damageDealt"`
	DamageTaken float64 `json:"damageTaken"`
	BehindDealt float64 `json:"behindDealt"`
	Swings      int     `json:"swings"`
}

type classAttributesDTO struct {
	Str int32 `json:"str"`
	Dex int32 `json:"dex"`
	Int int32 `json:"int"`
	Wis int32 `json:"wis"`
	Sta int32 `json:"sta"`
}

type classGritDTO struct {
	Cap               int     `json:"cap"`
	Opening           int     `json:"opening"`
	StarterSwing      int32   `json:"starterSwing"`
	SlamPerStack      float64 `json:"slamPerStack"`
	SlamCap           float64 `json:"slamCap"`
	RetaliatePerStack float64 `json:"retaliatePerStack"`
	SelfGuardGain     int     `json:"selfGuardGain"`
}

type classStartItemDTO struct {
	Slot string `json:"slot"`
	Name string `json:"name"`
}

type classSkillDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Effect       string  `json:"effect"`
	Level        int32   `json:"level"`
	OncePerFight bool    `json:"oncePerFight"`
	KeepsSwing   bool    `json:"keepsSwing"`
	Cooldown     int     `json:"cooldown"`
	ManaCost     int     `json:"manaCost"`
	Multiplier   float64 `json:"multiplier"`
	Duration     int     `json:"duration"`
	Tooltip      string  `json:"tooltip"`
	Target       string  `json:"target"`
	Component    string  `json:"component,omitempty"`
}

type classTemplateDTO struct {
	ID            string              `json:"id,omitempty"`
	Backstory     string              `json:"backstory,omitempty"`
	Origin        string              `json:"origin,omitempty"`
	Archetype     string              `json:"archetype,omitempty"`
	Race          string              `json:"race,omitempty"`
	Str           int32               `json:"str"`
	Dex           int32               `json:"dex"`
	Int           int32               `json:"int"`
	Wis           int32               `json:"wis"`
	Sta           int32               `json:"sta"`
	Mana          int32               `json:"mana"`
	Items         []classStartItemDTO `json:"startingItems,omitempty"`
	DefaultSkills []string            `json:"defaultSkills,omitempty"`
}

// classDTO is the full definition the creator editor reads and writes. It
// mirrors data/classes/*.yaml while keeping the editor's flat shape
// (attributes/mana/hotbar) so the client can bind fields directly.
type classDTO struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ArmorType     string            `json:"armorType"`
	CombatType    string            `json:"combatType"`
	School        string            `json:"school,omitempty"`
	Primary       string            `json:"primary"`
	SkillClass    string            `json:"skillClass"`
	Aliases       []string          `json:"aliases"`
	Also          []string          `json:"also,omitempty"`
	Races         []string          `json:"races"`
	PortraitClass string            `json:"portraitClass"`
	HPMultiplier  float64           `json:"hpMultiplier"`
	Caster        bool              `json:"caster"`
	HotbarCap     int               `json:"hotbarCap"`
	BraceCharges  int               `json:"braceCharges"`
	SlipCharges   int               `json:"slipCharges"`
	BoltCharges   int               `json:"boltCharges"`
	RigCharges    int               `json:"rigCharges"`
	InscribeBasic bool              `json:"inscribeBasic"`
	Order         int               `json:"order"`
	Balance       classBalanceDTO   `json:"balance"`
	Grit          *classGritDTO     `json:"grit,omitempty"`
	RaceBlurbs    map[string]string `json:"raceBlurbs,omitempty"`
	Talents       []string          `json:"talents,omitempty"`
	Skills        []classSkillDTO   `json:"skills"`

	// Flat template fields used by the editor.
	Attributes    classAttributesDTO  `json:"attributes"`
	Mana          int32               `json:"mana"`
	Hotbar        []string            `json:"hotbar"`
	TemplateID    string              `json:"templateId,omitempty"`
	Backstory     string              `json:"backstory,omitempty"`
	Origin        string              `json:"origin,omitempty"`
	Archetype     string              `json:"archetype,omitempty"`
	Race          string              `json:"race,omitempty"`
	StartingItems []classStartItemDTO `json:"startingItems,omitempty"`
	Template      *classTemplateDTO   `json:"template,omitempty"`
}

func defToDTO(d *classkit.Def) *classDTO {
	if d == nil {
		return nil
	}
	dto := &classDTO{
		ID:            d.ID,
		Name:          d.Name,
		Description:   d.Description,
		ArmorType:     d.ArmorType,
		CombatType:    d.CombatType,
		School:        d.School,
		Primary:       d.Primary,
		SkillClass:    d.SkillClass,
		Aliases:       append([]string{}, d.Aliases...),
		Also:          append([]string{}, d.Also...),
		Races:         append([]string{}, d.Races...),
		PortraitClass: d.PortraitClass,
		HPMultiplier:  d.HPMultiplier,
		Caster:        d.Caster,
		HotbarCap:     d.Hotbar,
		BraceCharges:  d.BraceCharges,
		SlipCharges:   d.SlipCharges,
		BoltCharges:   d.BoltCharges,
		RigCharges:    d.RigCharges,
		InscribeBasic: d.InscribeBasic,
		Order:         d.Order,
		Balance: classBalanceDTO{
			DamageDealt: d.Balance.DamageDealt,
			DamageTaken: d.Balance.DamageTaken,
			BehindDealt: d.Balance.BehindDealt,
			Swings:      d.Balance.Swings,
		},
		RaceBlurbs: cloneStringMap(d.RaceBlurbs),
		Talents:    append([]string{}, d.Talents...),
		Hotbar:     []string{},
		Skills:     make([]classSkillDTO, 0, len(d.Skills)),
	}
	if d.Grit != nil {
		dto.Grit = &classGritDTO{
			Cap:               d.Grit.Cap,
			Opening:           d.Grit.Opening,
			StarterSwing:      d.Grit.StarterSwing,
			SlamPerStack:      d.Grit.SlamPerStack,
			SlamCap:           d.Grit.SlamCap,
			RetaliatePerStack: d.Grit.RetaliatePerStack,
			SelfGuardGain:     d.Grit.SelfGuardGain,
		}
	}
	for _, s := range d.Skills {
		dto.Skills = append(dto.Skills, classSkillDTO{
			ID:           s.ID,
			Name:         s.Name,
			Effect:       s.Effect,
			Level:        s.Level,
			OncePerFight: s.OncePerFight,
			KeepsSwing:   s.KeepsSwing,
			Cooldown:     s.Cooldown,
			ManaCost:     s.ManaCost,
			Multiplier:   s.Multiplier,
			Duration:     s.Duration,
			Tooltip:      s.Tooltip,
			Target:       s.Target,
			Component:    s.Component,
		})
	}
	if d.Template != nil {
		t := d.Template
		dto.Attributes = classAttributesDTO{
			Str: t.Str, Dex: t.Dex, Int: t.Int, Wis: t.Wis, Sta: t.Sta,
		}
		dto.Mana = t.Mana
		dto.Hotbar = append([]string{}, t.DefaultSkills...)
		dto.TemplateID = t.ID
		dto.Backstory = t.Backstory
		dto.Origin = t.Origin
		dto.Archetype = t.Archetype
		dto.Race = t.Race
		dto.StartingItems = make([]classStartItemDTO, 0, len(t.Items))
		for _, it := range t.Items {
			dto.StartingItems = append(dto.StartingItems, classStartItemDTO{Slot: it.Slot, Name: it.Name})
		}
		dto.Template = &classTemplateDTO{
			ID:            t.ID,
			Backstory:     t.Backstory,
			Origin:        t.Origin,
			Archetype:     t.Archetype,
			Race:          t.Race,
			Str:           t.Str,
			Dex:           t.Dex,
			Int:           t.Int,
			Wis:           t.Wis,
			Sta:           t.Sta,
			Mana:          t.Mana,
			Items:         append([]classStartItemDTO{}, dto.StartingItems...),
			DefaultSkills: append([]string{}, t.DefaultSkills...),
		}
	}
	return dto
}

func (dto *classDTO) toDef() *classkit.Def {
	if dto == nil {
		return nil
	}
	d := &classkit.Def{
		ID:            strings.TrimSpace(dto.ID),
		Name:          dto.Name,
		Description:   dto.Description,
		ArmorType:     dto.ArmorType,
		CombatType:    dto.CombatType,
		School:        dto.School,
		Primary:       dto.Primary,
		SkillClass:    dto.SkillClass,
		Aliases:       append([]string{}, dto.Aliases...),
		Also:          append([]string{}, dto.Also...),
		Races:         append([]string{}, dto.Races...),
		PortraitClass: dto.PortraitClass,
		HPMultiplier:  dto.HPMultiplier,
		Caster:        dto.Caster,
		Hotbar:        dto.HotbarCap,
		BraceCharges:  dto.BraceCharges,
		SlipCharges:   dto.SlipCharges,
		BoltCharges:   dto.BoltCharges,
		RigCharges:    dto.RigCharges,
		InscribeBasic: dto.InscribeBasic,
		Order:         dto.Order,
		Balance: classkit.Row{
			DamageDealt: dto.Balance.DamageDealt,
			DamageTaken: dto.Balance.DamageTaken,
			BehindDealt: dto.Balance.BehindDealt,
			Swings:      dto.Balance.Swings,
		},
		RaceBlurbs: cloneStringMap(dto.RaceBlurbs),
		Talents:    append([]string{}, dto.Talents...),
		Skills:     make([]classkit.SkillSpec, 0, len(dto.Skills)),
	}
	if dto.Grit != nil {
		d.Grit = &classkit.Grit{
			Cap:               dto.Grit.Cap,
			Opening:           dto.Grit.Opening,
			StarterSwing:      dto.Grit.StarterSwing,
			SlamPerStack:      dto.Grit.SlamPerStack,
			SlamCap:           dto.Grit.SlamCap,
			RetaliatePerStack: dto.Grit.RetaliatePerStack,
			SelfGuardGain:     dto.Grit.SelfGuardGain,
		}
	}
	for _, s := range dto.Skills {
		d.Skills = append(d.Skills, classkit.SkillSpec{
			ID:           s.ID,
			Name:         s.Name,
			Effect:       s.Effect,
			Level:        s.Level,
			OncePerFight: s.OncePerFight,
			KeepsSwing:   s.KeepsSwing,
			Cooldown:     s.Cooldown,
			ManaCost:     s.ManaCost,
			Multiplier:   s.Multiplier,
			Duration:     s.Duration,
			Tooltip:      s.Tooltip,
			Target:       s.Target,
			Component:    s.Component,
		})
	}

	if dto.Template != nil {
		d.Template = dto.Template.toTemplate()
	}
	if hasFlatTemplate(dto) {
		if d.Template == nil {
			d.Template = &classkit.Template{}
		}
		t := d.Template
		if dto.TemplateID != "" {
			t.ID = dto.TemplateID
		}
		t.Backstory = dto.Backstory
		t.Origin = dto.Origin
		t.Archetype = dto.Archetype
		t.Race = dto.Race
		t.Str = dto.Attributes.Str
		t.Dex = dto.Attributes.Dex
		t.Int = dto.Attributes.Int
		t.Wis = dto.Attributes.Wis
		t.Sta = dto.Attributes.Sta
		t.Mana = dto.Mana
		if dto.StartingItems != nil {
			t.Items = make([]classkit.StartItem, 0, len(dto.StartingItems))
			for _, it := range dto.StartingItems {
				t.Items = append(t.Items, classkit.StartItem{Slot: it.Slot, Name: it.Name})
			}
		}
		if dto.Hotbar != nil {
			t.DefaultSkills = append([]string{}, dto.Hotbar...)
		}
	}
	if d.Template != nil && strings.TrimSpace(d.Template.ID) == "" && strings.TrimSpace(d.ID) != "" {
		d.Template.ID = "tpl-" + strings.ToLower(strings.TrimSpace(d.ID))
	}
	return d
}

func (dto *classTemplateDTO) toTemplate() *classkit.Template {
	if dto == nil {
		return nil
	}
	t := &classkit.Template{
		ID:            dto.ID,
		Backstory:     dto.Backstory,
		Origin:        dto.Origin,
		Archetype:     dto.Archetype,
		Race:          dto.Race,
		Str:           dto.Str,
		Dex:           dto.Dex,
		Int:           dto.Int,
		Wis:           dto.Wis,
		Sta:           dto.Sta,
		Mana:          dto.Mana,
		DefaultSkills: append([]string{}, dto.DefaultSkills...),
	}
	if dto.Items != nil {
		t.Items = make([]classkit.StartItem, 0, len(dto.Items))
		for _, it := range dto.Items {
			t.Items = append(t.Items, classkit.StartItem{Slot: it.Slot, Name: it.Name})
		}
	}
	return t
}

func hasFlatTemplate(dto *classDTO) bool {
	if dto.TemplateID != "" || dto.Backstory != "" || dto.Origin != "" ||
		dto.Archetype != "" || dto.Race != "" || dto.Mana != 0 ||
		len(dto.Hotbar) > 0 || len(dto.StartingItems) > 0 {
		return true
	}
	a := dto.Attributes
	return a.Str != 0 || a.Dex != 0 || a.Int != 0 || a.Wis != 0 || a.Sta != 0
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func writeClassError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, classkit.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case classkit.IsValidationError(err):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// Catalog serves GET /api/classes. The public payload is unchanged, but a
// creator/admin token on the same path receives the full editor definitions.
// This keeps the existing public route and avoids a duplicate Gin registration.
func (h *ClassesHandler) Catalog(c *gin.Context) {
	if usr, ok := c.Get("user"); ok {
		if user, ok := usr.(interface{ IsCreator() bool }); ok && user.IsCreator() {
			h.List(c)
			return
		}
	}
	GetClasses(c)
}

// List returns every class with balance and template data.
func (h *ClassesHandler) List(c *gin.Context) {
	defs := classkit.All()
	out := make([]*classDTO, 0, len(defs))
	for _, d := range defs {
		if d == nil {
			continue
		}
		out = append(out, defToDTO(d))
	}
	c.JSON(http.StatusOK, out)
}

// GetClassByID returns one full class definition.
func (h *ClassesHandler) GetClassByID(c *gin.Context) {
	d := classkit.FindDef(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	c.JSON(http.StatusOK, defToDTO(d))
}

// Create writes a new class YAML file.
func (h *ClassesHandler) Create(c *gin.Context) {
	var dto classDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d := dto.toDef()
	if err := classkit.ValidateDef(d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if existing := classkit.FindDef(d.ID); existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("class %s already exists", existing.ID)})
		return
	}
	if err := classkit.SaveDef(classkit.ActiveDir(), d); err != nil {
		writeClassError(c, err)
		return
	}
	saved := classkit.FindDef(d.ID)
	if saved == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "class saved but not found in catalog"})
		return
	}
	c.JSON(http.StatusOK, defToDTO(saved))
}

// Update rewrites an existing class YAML file. The path id wins over any id in
// the request body.
func (h *ClassesHandler) Update(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if classkit.FindDef(id) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	var dto classDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d := dto.toDef()
	bodyID := strings.TrimSpace(d.ID)
	d.ID = id
	// The path id wins. If the body carried the same id in skill_class, clear
	// it so the catalog does not resolve the discarded body id as an alias.
	if bodyID != "" && !strings.EqualFold(bodyID, id) && strings.EqualFold(d.SkillClass, bodyID) {
		d.SkillClass = ""
	}
	if err := classkit.ValidateDef(d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := classkit.SaveDef(classkit.ActiveDir(), d); err != nil {
		writeClassError(c, err)
		return
	}
	saved := classkit.FindDef(id)
	if saved == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "class saved but not found in catalog"})
		return
	}
	c.JSON(http.StatusOK, defToDTO(saved))
}

// Delete removes a class YAML file.
func (h *ClassesHandler) Delete(c *gin.Context) {
	if err := classkit.DeleteDef(classkit.ActiveDir(), c.Param("id")); err != nil {
		writeClassError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

type classDuplicateRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Duplicate copies an existing class. The optional body may set the new id and
// name; otherwise the id is <source>_copy and the name gets a "(copy)" suffix.
func (h *ClassesHandler) Duplicate(c *gin.Context) {
	source := classkit.FindDef(c.Param("id"))
	if source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	var req classDuplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	newID := strings.TrimSpace(req.ID)
	if newID == "" {
		newID = uniqueClassID(source.ID + "_copy")
	} else if existing := classkit.FindDef(newID); existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("class %s already exists", existing.ID)})
		return
	}
	copyDef := source
	copyDef.ID = newID
	// A duplicate must not keep the source's alternate ids or template key, and
	// its kit skills need fresh ids so the per-id skill cache cannot collide.
	copyDef.Aliases = nil
	copyDef.Also = nil
	renamedSkills := map[string]string{}
	for i := range copyDef.Skills {
		oldID := copyDef.Skills[i].ID
		base := oldID
		if idx := strings.Index(oldID, "_"); idx >= 0 {
			base = oldID[idx+1:]
		}
		newSkillID := newID + "_" + base
		copyDef.Skills[i].ID = newSkillID
		renamedSkills[oldID] = newSkillID
	}
	if copyDef.Template != nil {
		copyDef.Template.ID = "tpl-" + strings.ToLower(newID)
		for i, skillID := range copyDef.Template.DefaultSkills {
			if renamed, ok := renamedSkills[skillID]; ok {
				copyDef.Template.DefaultSkills[i] = renamed
			}
		}
	}
	if strings.TrimSpace(req.Name) != "" {
		copyDef.Name = req.Name
	} else {
		copyDef.Name = source.Name + " (copy)"
	}
	if err := classkit.SaveDef(classkit.ActiveDir(), copyDef); err != nil {
		writeClassError(c, err)
		return
	}
	saved := classkit.FindDef(newID)
	if saved == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "class saved but not found in catalog"})
		return
	}
	c.JSON(http.StatusOK, defToDTO(saved))
}

func uniqueClassID(base string) string {
	id := base
	for i := 2; classkit.FindDef(id) != nil; i++ {
		id = fmt.Sprintf("%s_%d", base, i)
	}
	return id
}
