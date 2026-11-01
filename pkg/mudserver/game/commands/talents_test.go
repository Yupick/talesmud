package commands_test

import (
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

func TestTalentsCommandListAndSpend(t *testing.T) {
	g, facade := newSocialTestGame(t)
	user, char := storeSocialPlayer(t, facade, "user-tal", "ref-tal", "char-tal", "Talin", "room-1")
	char.Class = characters.Class{ID: "warrior", Name: "Sentinel"}
	char.Level = 5
	char.Talents.Points = 1
	if err := facade.CharactersService().Update(char.ID, char); err != nil {
		t.Fatalf("prep character: %v", err)
	}
	g.ConnectUserSession(user)
	g.SetUserSessionCharacter(user, char)
	_ = drainSocialMessages(g.SendMessage())

	list := &messages.Message{FromUser: user, Character: char, Data: "talents"}
	if !(&commands.TalentsCommand{}).Execute(g, list) {
		t.Fatal("talents did not handle")
	}
	out := drainSocialMessages(g.SendMessage())
	text := strings.Join(friendReplyTexts(out), "\n")
	if !strings.Contains(text, "Unspent Points: 1") {
		t.Fatalf("list missing points: %q", text)
	}
	if !strings.Contains(text, "talent_power_strike") {
		t.Fatalf("list missing warrior talent: %q", text)
	}
	if strings.Contains(text, "talent_shadow_step") {
		t.Fatalf("list must not show shadow-only talent: %q", text)
	}

	spend := &messages.Message{FromUser: user, Character: char, Data: "talents spend critical_eye"}
	if !(&commands.TalentsCommand{}).Execute(g, spend) {
		t.Fatal("talents spend did not handle")
	}
	out = drainSocialMessages(g.SendMessage())
	text = strings.Join(friendReplyTexts(out), "\n")
	if !strings.Contains(text, "Ojo Crítico") {
		t.Fatalf("spend confirmation missing talent name: %q", text)
	}
	if char.Talents.Points != 0 || char.TalentRank("talent_critical_eye") != 1 {
		t.Fatalf("spend state: points=%d ranks=%+v", char.Talents.Points, char.Talents.Spent)
	}

	loaded, err := facade.CharactersService().FindByID(char.ID)
	if err != nil || loaded == nil {
		t.Fatalf("reload character: %v", err)
	}
	if loaded.TalentRank("talent_critical_eye") != 1 {
		t.Fatalf("spent talent not persisted: %+v", loaded.Talents)
	}
}

func TestTalentsCommandRejectsForeignTalent(t *testing.T) {
	g, facade := newSocialTestGame(t)
	user, char := storeSocialPlayer(t, facade, "user-tal2", "ref-tal2", "char-tal2", "Rin", "room-1")
	char.Class = characters.Class{ID: "warrior", Name: "Sentinel"}
	char.Talents.Points = 1
	if err := facade.CharactersService().Update(char.ID, char); err != nil {
		t.Fatalf("prep character: %v", err)
	}
	g.ConnectUserSession(user)
	g.SetUserSessionCharacter(user, char)
	_ = drainSocialMessages(g.SendMessage())

	msg := &messages.Message{FromUser: user, Character: char, Data: "talents spend shadow_step"}
	(&commands.TalentsCommand{}).Execute(g, msg)
	out := drainSocialMessages(g.SendMessage())
	text := strings.Join(friendReplyTexts(out), "\n")
	if !strings.Contains(text, "cannot learn") {
		t.Fatalf("expected class refusal, got %q", text)
	}
	if char.Talents.Points != 1 || char.TalentRank("talent_shadow_step") != 0 {
		t.Fatal("foreign talent must not be bought")
	}
}

func TestPrestigeCommandRequiresMaxLevel(t *testing.T) {
	g, facade := newSocialTestGame(t)
	user, char := storeSocialPlayer(t, facade, "user-pres0", "ref-pres0", "char-pres0", "Low", "room-1")
	char.Class = characters.Class{ID: "warrior", Name: "Sentinel"}
	char.Level = 49
	g.ConnectUserSession(user)
	g.SetUserSessionCharacter(user, char)
	_ = drainSocialMessages(g.SendMessage())

	msg := &messages.Message{FromUser: user, Character: char, Data: "prestige confirm"}
	(&commands.PrestigeCommand{}).Execute(g, msg)
	out := drainSocialMessages(g.SendMessage())
	text := strings.Join(friendReplyTexts(out), "\n")
	if !strings.Contains(text, "must reach level") {
		t.Fatalf("expected level requirement, got %q", text)
	}
	if char.Level != 49 || char.Talents.Prestige != 0 {
		t.Fatal("prestige must not run below max level")
	}
}

func TestPrestigeCommandResetsAndGrantsBonus(t *testing.T) {
	g, facade := newSocialTestGame(t)
	user, char := storeSocialPlayer(t, facade, "user-pres1", "ref-pres1", "char-pres1", "High", "room-1")
	char.Class = characters.Class{ID: "warrior", Name: "Sentinel"}
	char.Level = 50
	char.XP = 30000
	char.MaxHitPoints = 400
	char.CurrentHitPoints = 400
	if err := facade.CharactersService().Update(char.ID, char); err != nil {
		t.Fatalf("prep character: %v", err)
	}
	g.ConnectUserSession(user)
	g.SetUserSessionCharacter(user, char)
	_ = drainSocialMessages(g.SendMessage())

	msg := &messages.Message{FromUser: user, Character: char, Data: "prestige confirm"}
	if !(&commands.PrestigeCommand{}).Execute(g, msg) {
		t.Fatal("prestige did not handle")
	}
	out := drainSocialMessages(g.SendMessage())
	text := strings.Join(friendReplyTexts(out), "\n")
	if !strings.Contains(text, "Prestige 1") {
		t.Fatalf("expected prestige confirmation, got %q", text)
	}
	if char.Level != 1 || char.XP != 0 {
		t.Fatalf("level/xp = %d/%d, want 1/0", char.Level, char.XP)
	}
	if char.Talents.Prestige != 1 || char.Talents.Points != 1 {
		t.Fatalf("prestige/points = %d/%d, want 1/1", char.Talents.Prestige, char.Talents.Points)
	}
	if char.PrestigeMultiplier() != 1.05 {
		t.Fatalf("multiplier = %v, want 1.05", char.PrestigeMultiplier())
	}

	loaded, err := facade.CharactersService().FindByID(char.ID)
	if err != nil || loaded == nil {
		t.Fatalf("reload character: %v", err)
	}
	if loaded.Level != 1 || loaded.Talents.Prestige != 1 || loaded.Talents.Points != 1 {
		t.Fatalf("prestige not persisted: %+v", loaded.Talents)
	}
}
