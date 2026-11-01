package commands

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/leveling"
)

// dmCaptureGame implements only the GameCtrl calls the DM notifiers use. The
// embedded interface is nil on purpose: any other method would panic, which
// keeps the tests honest about the dependency surface.
type dmCaptureGame struct {
	def.GameCtrl
	send    chan interface{}
	players []def.OnlinePlayer
	events  []def.DMEvent
}

func newDMCaptureGame() *dmCaptureGame {
	return &dmCaptureGame{
		send: make(chan interface{}, 4),
		players: []def.OnlinePlayer{
			{UserID: "u1", CharacterID: "c1", RoomID: "R1", CharacterName: "Hero"},
		},
	}
}

func (g *dmCaptureGame) SendMessage() chan interface{} { return g.send }

func (g *dmCaptureGame) GetOnlinePlayers() []def.OnlinePlayer { return g.players }

func (g *dmCaptureGame) NotifyDM(e def.DMEvent) { g.events = append(g.events, e) }

func TestNotifyLevelUpDM(t *testing.T) {
	g := newDMCaptureGame()
	notifyLevelUp(g, "u1", &leveling.LevelUpResult{OldLevel: 4, NewLevel: 5, LevelsGained: 1})

	if len(g.events) != 1 {
		t.Fatalf("events = %d, want 1", len(g.events))
	}
	e := g.events[0]
	if e.Kind != "levelUp" || e.UserID != "u1" || e.CharacterID != "c1" || e.RoomID != "R1" {
		t.Errorf("unexpected event: %+v", e)
	}
	if e.Facts["level"] != "5" {
		t.Errorf("level fact = %q, want 5", e.Facts["level"])
	}
}

func TestNotifyLevelUpDMIgnoresZeroGain(t *testing.T) {
	g := newDMCaptureGame()
	notifyLevelUp(g, "u1", &leveling.LevelUpResult{NewLevel: 5, LevelsGained: 0})
	if len(g.events) != 0 {
		t.Fatalf("events = %d, want 0", len(g.events))
	}
}

func TestNotifyQuestCompletedDM(t *testing.T) {
	g := newDMCaptureGame()
	char := &characters.Character{
		Entity:      &entities.Entity{ID: "c1"},
		BelongsUser: traits.BelongsUser{BelongsUserID: "u1"},
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "R1"},
	}
	quest := &quests.Quest{Entity: &entities.Entity{ID: "q1"}, Name: "The Lost Ledger"}
	notifyQuestCompletedDM(g, char, quest)

	if len(g.events) != 1 {
		t.Fatalf("events = %d, want 1", len(g.events))
	}
	e := g.events[0]
	if e.Kind != "questCompleted" || e.UserID != "u1" || e.CharacterID != "c1" || e.RoomID != "R1" {
		t.Errorf("unexpected event: %+v", e)
	}
	if e.Facts["questName"] != "The Lost Ledger" {
		t.Errorf("questName fact = %q", e.Facts["questName"])
	}
}
