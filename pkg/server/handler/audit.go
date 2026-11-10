package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/audit"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/service"
	"github.com/talesmud/talesmud/pkg/worldindex"
)

type auditRoute struct {
	entityType string
	action     string
}

// auditRoutes are the creator CRUD writes. Reads, previews, and validates stay out.
var auditRoutes = map[string]auditRoute{
	"POST /api/rooms":                     {entityType: "rooms", action: "create"},
	"PUT /api/rooms/:id":                  {entityType: "rooms", action: "update"},
	"DELETE /api/rooms/:id":               {entityType: "rooms", action: "delete"},
	"POST /api/items":                     {entityType: "items", action: "create"},
	"PUT /api/items/:id":                  {entityType: "items", action: "update"},
	"DELETE /api/items/:id":               {entityType: "items", action: "delete"},
	"POST /api/npcs":                      {entityType: "npcs", action: "create"},
	"PUT /api/npcs/:id":                   {entityType: "npcs", action: "update"},
	"DELETE /api/npcs/:id":                {entityType: "npcs", action: "delete"},
	"POST /api/loottables":                {entityType: "loottables", action: "create"},
	"PUT /api/loottables/:id":             {entityType: "loottables", action: "update"},
	"DELETE /api/loottables/:id":          {entityType: "loottables", action: "delete"},
	"POST /api/spawners":                  {entityType: "spawners", action: "create"},
	"PUT /api/spawners/:id":               {entityType: "spawners", action: "update"},
	"DELETE /api/spawners/:id":            {entityType: "spawners", action: "delete"},
	"POST /api/dialogs":                   {entityType: "dialogs", action: "create"},
	"PUT /api/dialogs/:id":                {entityType: "dialogs", action: "update"},
	"DELETE /api/dialogs/:id":             {entityType: "dialogs", action: "delete"},
	"POST /api/quests":                    {entityType: "quests", action: "create"},
	"PUT /api/quests/:id":                 {entityType: "quests", action: "update"},
	"DELETE /api/quests/:id":              {entityType: "quests", action: "delete"},
	"POST /api/scripts":                   {entityType: "scripts", action: "create"},
	"PUT /api/scripts/:id":                {entityType: "scripts", action: "update"},
	"DELETE /api/scripts/:id":             {entityType: "scripts", action: "delete"},
	"POST /api/skills":                    {entityType: "skills", action: "create"},
	"PUT /api/skills/:id":                 {entityType: "skills", action: "update"},
	"DELETE /api/skills/:id":              {entityType: "skills", action: "delete"},
	"POST /api/spells":                    {entityType: "skills", action: "create"},
	"PUT /api/spells/:id":                 {entityType: "skills", action: "update"},
	"DELETE /api/spells/:id":              {entityType: "skills", action: "delete"},
	"POST /api/classes":                   {entityType: "classes", action: "create"},
	"PUT /api/classes/:id":                {entityType: "classes", action: "update"},
	"DELETE /api/classes/:id":             {entityType: "classes", action: "delete"},
	"POST /api/classes/:id/duplicate":     {entityType: "classes", action: "create"},
	"POST /api/character-templates":       {entityType: "character-templates", action: "create"},
	"PUT /api/character-templates/:id":    {entityType: "character-templates", action: "update"},
	"DELETE /api/character-templates/:id": {entityType: "character-templates", action: "delete"},
	"PUT /api/settings":                   {entityType: "settings", action: "update"},
}

// AuditWrites records a successful creator CRUD write. A log failure does not change the response.
// A successful POST, PUT, PATCH, or DELETE also drops the cached search index, including
// creator writes that this middleware does not record.
func AuditWrites(svc service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer invalidateSearch(c)
		spec, ok := auditRoutes[c.Request.Method+" "+c.FullPath()]
		if !ok || svc == nil {
			c.Next()
			return
		}
		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
		}
		entityID := c.Param("id")
		if spec.entityType == "settings" {
			entityID = "server-settings"
		}
		var before json.RawMessage
		if c.Request.Method != http.MethodPost {
			raw, err := loadAuditRaw(svc, spec.entityType, entityID)
			if err != nil {
				log.WithError(err).WithField("entityType", spec.entityType).Error("audit snapshot failed")
				c.Next()
				return
			}
			before = raw
		}
		cap := &bodyCapture{ResponseWriter: c.Writer}
		c.Writer = cap
		c.Next()
		if c.Writer.Status() >= 400 {
			return
		}
		if c.Request.Method == http.MethodPost {
			entityID = idFromJSON(cap.body.Bytes())
		}
		if entityID == "" {
			log.WithField("path", c.FullPath()).Error("audit skipped: no entity id")
			return
		}
		var after json.RawMessage
		if c.Request.Method != http.MethodDelete {
			raw, err := loadAuditRaw(svc, spec.entityType, entityID)
			if err != nil {
				log.WithError(err).Error("audit after-snapshot failed")
				return
			}
			after = raw
		}
		if _, err := svc.Record(service.AuditRecord{
			Actor:      actorFrom(c),
			Action:     spec.action,
			EntityType: spec.entityType,
			EntityID:   entityID,
			Before:     before,
			After:      after,
			Source:     audit.SourceCreator,
			Undoable:   true,
			Summary:    spec.action + " " + spec.entityType + " " + entityID,
		}); err != nil {
			log.WithError(err).Error("audit record failed")
		}
		_ = reqBody
	}
}

func invalidateSearch(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return
	}
	if c.Writer == nil || c.Writer.Status() >= 400 {
		return
	}
	worldindex.Live.Invalidate()
}

// ListAudit returns recent audit rows for creators.
func ListAudit(svc service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "audit log is not available"})
			return
		}
		limit, _ := strconv.Atoi(c.Query("limit"))
		rows, err := svc.List(c.Query("entityType"), c.Query("entityId"), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []*audit.Entry{}
		}
		c.JSON(http.StatusOK, rows)
	}
}

// UndoAudit restores a creator row or runs an ops inverse. Admin only.
func UndoAudit(g *game.Game, svc service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "audit log is not available"})
			return
		}
		entry, err := svc.Get(c.Param("id"))
		if err != nil || entry == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit entry not found"})
			return
		}
		actor := actorFrom(c)
		if entry.Source == audit.SourceOps {
			undoOps(c, g, svc, entry, actor)
			return
		}
		if entry.EntityType == "classes" {
			undoClass(c, svc, entry, actor)
			return
		}
		undo, err := svc.UndoCRUD(c.Param("id"), actor)
		if err != nil {
			writeCoded(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"summary":  undo.Summary,
			"auditId":  undo.ID,
			"undoable": undo.Undoable,
		})
	}
}

// undoClass reverses a class-definition audit entry by restoring or deleting
// the YAML file. It mirrors service.UndoCRUD's conflict checks but uses the
// classkit catalog instead of a SQLite table.
func undoClass(c *gin.Context, svc service.AuditService, entry *audit.Entry, actor service.AuditActor) {
	if entry.UndoneBy != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this change was already undone"})
		return
	}
	if !entry.Undoable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this change cannot be undone"})
		return
	}
	current := classAuditRaw(entry.EntityID)
	if !jsonRawEqual(current, entry.After) {
		c.JSON(http.StatusConflict, gin.H{"error": "the entity changed since this change; undo was refused"})
		return
	}
	if isEmptyJSON(entry.Before) {
		if err := classkit.DeleteDef(classkit.ActiveDir(), entry.EntityID); err != nil && !errors.Is(err, classkit.ErrNotFound) {
			writeClassError(c, err)
			return
		}
	} else {
		var dto classDTO
		if err := json.Unmarshal(entry.Before, &dto); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "stored class snapshot is invalid"})
			return
		}
		d := dto.toDef()
		d.ID = entry.EntityID
		if bodyID := strings.TrimSpace(dto.ID); bodyID != "" && !strings.EqualFold(bodyID, d.ID) && strings.EqualFold(d.SkillClass, bodyID) {
			d.SkillClass = ""
		}
		if err := classkit.SaveDef(classkit.ActiveDir(), d); err != nil {
			writeClassError(c, err)
			return
		}
	}
	restored := classAuditRaw(entry.EntityID)
	undo, err := svc.Record(service.AuditRecord{
		Actor:      actor,
		Action:     "undo",
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Before:     current,
		After:      restored,
		Source:     audit.SourceCreator,
		Undoable:   true,
		Summary:    "undo " + entry.Action + " " + entry.EntityType + " " + entry.EntityID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "the undo succeeded but the audit log failed"})
		return
	}
	if err := svc.MarkUndone(entry.ID, undo.ID); err != nil {
		log.WithError(err).Error("audit mark undone failed")
	}
	c.JSON(http.StatusOK, gin.H{
		"summary":  undo.Summary,
		"auditId":  undo.ID,
		"undoable": undo.Undoable,
	})
}

func classAuditRaw(entityID string) json.RawMessage {
	d := classkit.FindDef(entityID)
	if d == nil {
		return nil
	}
	raw, err := json.Marshal(defToDTO(d))
	if err != nil {
		return nil
	}
	return raw
}

func isEmptyJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func jsonRawEqual(a, b json.RawMessage) bool {
	return bytes.Equal(compactJSON(a), compactJSON(b))
}

func compactJSON(raw json.RawMessage) []byte {
	if isEmptyJSON(raw) {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

func undoOps(c *gin.Context, g *game.Game, svc service.AuditService, entry *audit.Entry, actor service.AuditActor) {
	if entry.UndoneBy != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this change was already undone"})
		return
	}
	if !entry.Undoable || entry.Inverse == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this change cannot be undone"})
		return
	}
	if g == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "game is not running"})
		return
	}
	var res *game.OpResult
	var opErr error
	if err := g.Call(func() {
		res, opErr = g.ApplyInverse(entry.Inverse)
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if opErr != nil {
		writeCoded(c, opErr)
		return
	}
	undo, err := svc.Record(service.AuditRecord{
		Actor:      actor,
		Action:     "undo",
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Before:     res.Before,
		After:      res.After,
		Source:     audit.SourceOps,
		Undoable:   res.Undoable,
		Summary:    res.Summary,
		Inverse:    res.Inverse,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "the undo succeeded but the audit log failed"})
		return
	}
	if err := svc.MarkUndone(entry.ID, undo.ID); err != nil {
		log.WithError(err).Error("audit mark undone failed")
	}
	c.JSON(http.StatusOK, gin.H{
		"summary":  res.Summary,
		"auditId":  undo.ID,
		"undoable": res.Undoable,
		"result":   res.Detail,
	})
}

func actorFrom(c *gin.Context) service.AuditActor {
	usr, ok := c.Get("user")
	if !ok {
		return service.AuditActor{Name: "unknown"}
	}
	user, ok := usr.(*entities.User)
	if !ok || user == nil {
		return service.AuditActor{Name: "unknown"}
	}
	name := user.Nickname
	if name == "" {
		name = user.Name
	}
	if name == "" {
		name = user.Username
	}
	if name == "" {
		name = user.ID
	}
	return service.AuditActor{UserID: user.ID, Name: name}
}

// loadAuditRaw snapshots an entity for the change log. Class definitions are
// YAML files rather than SQLite rows, so they are serialized from the catalog
// using the same DTO as the class CRUD handler.
func loadAuditRaw(svc service.AuditService, entityType, entityID string) (json.RawMessage, error) {
	if entityType == "classes" {
		return classAuditRaw(entityID), nil
	}
	return svc.LoadRaw(entityType, entityID)
}

func idFromJSON(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var probe map[string]interface{}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	if id, ok := probe["id"].(string); ok {
		return id
	}
	return ""
}

type bodyCapture struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	_, _ = w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

type codedError interface {
	error
	StatusCode() int
}

func writeCoded(c *gin.Context, err error) {
	if err == nil {
		return
	}
	status := http.StatusInternalServerError
	var coded codedError
	if ok := asCoded(err, &coded); ok && coded.StatusCode() != 0 {
		status = coded.StatusCode()
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func asCoded(err error, dest *codedError) bool {
	switch e := err.(type) {
	case codedError:
		*dest = e
		return true
	default:
		return false
	}
}
