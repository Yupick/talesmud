package commands_test

import (
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

func channelTestCharacter() *characters.Character {
	return &characters.Character{Entity: &entities.Entity{ID: "char-1"}, Name: "Aster"}
}

func TestChannelsRequireCharacter(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}

	msg := &messages.Message{FromUser: user, Data: "general hello"}
	if !(&commands.ChannelsCommand{}).Execute(g, msg) {
		t.Fatal("channel command did not handle a known channel")
	}

	responses := partyResponseMessages(drainPartyMessages(g.SendMessage()))
	if len(responses) != 1 || responses[0].Message != "You must select a character first." {
		t.Fatalf("expected select-character reply, got %#v", responses)
	}
}

func TestChannelsGeneralBroadcastsGlobally(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}
	character := channelTestCharacter()

	msg := &messages.Message{FromUser: user, Character: character, Data: "general hello world"}
	if !(&commands.ChannelsCommand{}).Execute(g, msg) {
		t.Fatal("general channel not handled")
	}

	var found *messages.MessageResponse
	for _, out := range drainPartyMessages(g.SendMessage()) {
		if rsp, ok := out.(messages.MessageResponse); ok {
			found = &rsp
		}
	}
	if found == nil {
		t.Fatal("expected a broadcast response")
	}
	if found.Audience != messages.MessageAudienceGlobal {
		t.Fatalf("expected global audience, got %v", found.Audience)
	}
	if found.Type != messages.MessageTypeDefault {
		t.Fatalf("expected default type, got %v", found.Type)
	}
	if found.Username != "Aster" {
		t.Fatalf("expected username Aster, got %q", found.Username)
	}
	if found.Message != "[General] Aster: hello world" {
		t.Fatalf("unexpected message text %q", found.Message)
	}
	if found.Channel != "general" {
		t.Fatalf("expected channel tag general, got %q", found.Channel)
	}
}

func TestChannelsAdminRestrictedForPlayers(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}
	character := channelTestCharacter()

	msg := &messages.Message{FromUser: user, Character: character, Data: "admin secret plans"}
	if !(&commands.ChannelsCommand{}).Execute(g, msg) {
		t.Fatal("admin channel not handled")
	}

	for _, out := range drainPartyMessages(g.SendMessage()) {
		rsp, ok := out.(messages.MessageResponse)
		if !ok {
			continue
		}
		if rsp.Audience == messages.MessageAudienceGlobal {
			t.Fatalf("non-admin must not broadcast, got %#v", rsp)
		}
		if !strings.Contains(rsp.Message, "Admin channel is restricted.") {
			t.Fatalf("expected restriction reply, got %q", rsp.Message)
		}
	}
}

func TestChannelsAdminBroadcastsForAdmins(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}, Role: entities.RoleAdmin}
	character := channelTestCharacter()

	// Reaches the admin channel through the "chat" prefix.
	msg := &messages.Message{FromUser: user, Character: character, Data: "chat admin secret plans"}
	if !(&commands.ChannelsCommand{}).Execute(g, msg) {
		t.Fatal("admin channel not handled")
	}

	var found *messages.MessageResponse
	for _, out := range drainPartyMessages(g.SendMessage()) {
		if rsp, ok := out.(messages.MessageResponse); ok {
			found = &rsp
		}
	}
	if found == nil || found.Audience != messages.MessageAudienceGlobal {
		t.Fatalf("expected admin global broadcast, got %#v", found)
	}
	if found.Message != "[Admin] Aster: secret plans" || found.Channel != "admin" {
		t.Fatalf("unexpected admin message %#v", found)
	}
}

func TestChannelsEmptyMessageShowsUsage(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}
	character := channelTestCharacter()

	for _, data := range []string{"general", "chat help"} {
		if !(&commands.ChannelsCommand{}).Execute(g, &messages.Message{FromUser: user, Character: character, Data: data}) {
			t.Fatalf("%q should be handled", data)
		}
		responses := partyResponseMessages(drainPartyMessages(g.SendMessage()))
		if len(responses) != 1 || !strings.Contains(responses[0].Message, "Channels:") {
			t.Fatalf("expected usage for %q, got %#v", data, responses)
		}
	}
}

func TestChannelsUnknownIsNotIntercepted(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}
	character := channelTestCharacter()

	msg := &messages.Message{FromUser: user, Character: character, Data: "foo hola"}
	if (&commands.ChannelsCommand{}).Execute(g, msg) {
		t.Fatal("unknown channel must not be intercepted")
	}
	if got := drainPartyMessages(g.SendMessage()); len(got) != 0 {
		t.Fatalf("unknown channel must not emit messages, got %#v", got)
	}
}

// TestChannelsDoNotStealHelpOrTrade guards the alias decision: the existing
// help and merchant-list commands must keep owning "help" and "trade".
func TestChannelsDoNotStealHelpOrTrade(t *testing.T) {
	g, _ := newPartyCommandTestGame(t)
	user := &entities.User{Entity: &entities.Entity{ID: "user-1"}}
	character := channelTestCharacter()

	if !g.CommandProcessor.Process(g, &messages.Message{FromUser: user, Character: character, Data: "help"}) {
		t.Fatal("help should still be handled")
	}
	helpReplies := partyResponseMessages(drainPartyMessages(g.SendMessage()))
	if len(helpReplies) != 1 || !strings.Contains(helpReplies[0].Message, "List of all global commands") {
		t.Fatalf("expected the help command listing, got %#v", helpReplies)
	}

	if !g.CommandProcessor.Process(g, &messages.Message{FromUser: user, Character: character, Data: "trade"}) {
		t.Fatal("trade should still be handled")
	}
	tradeReplies := partyResponseMessages(drainPartyMessages(g.SendMessage()))
	if len(tradeReplies) != 1 || !strings.Contains(tradeReplies[0].Message, "You are not in a room.") {
		t.Fatalf("expected the merchant list command, got %#v", tradeReplies)
	}
}
