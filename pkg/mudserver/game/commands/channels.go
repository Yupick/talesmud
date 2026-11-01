package commands

import (
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// ChannelsCommand sends player messages to the global chat channels.
//
// Supported channels: general, trade, lfg, help, roleplay and admin.
//
// The canonical entry point is the "chat" prefix:
//
//	chat general anyone around?
//	chat trade WTB iron ore
//
// Direct shortcuts are also registered for the channels whose names are not
// already owned by another command, so "general anyone?" and "lfg need a
// healer" work without the prefix. The words "help" and "trade" are already
// taken by the existing help command and the merchant-list command, so those
// two channels are only reachable through "chat help ..." / "chat trade ...".
// This keeps the legacy "help" and bare "trade" commands intact.
type ChannelsCommand struct{}

// Key returns the command key matcher.
func (command *ChannelsCommand) Key() CommandKey { return &StartsWithCommandKey{} }

// channelOrder is the display order for the usage line.
var channelOrder = []string{"general", "trade", "lfg", "help", "roleplay", "admin"}

// channelTitles maps a lower-case channel name to its display title.
var channelTitles = map[string]string{
	"general":  "General",
	"trade":    "Trade",
	"lfg":      "LFG",
	"help":     "Help",
	"roleplay": "Roleplay",
	"admin":    "Admin",
}

// channelUsage returns the short usage line with the available channels.
func channelUsage() string {
	return "Usage: chat <channel> <message>. Channels: " + strings.Join(channelOrder, ", ")
}

// Execute sends the message to the requested global channel.
//
// An input whose first word is not a recognized channel (for example
// "foo hola") is not intercepted: it returns false so the room chat keeps
// handling it.
func (command *ChannelsCommand) Execute(game def.GameCtrl, message *messages.Message) bool {
	if message == nil {
		return false
	}

	channel, text, ok := parseChannelInput(message.Data)
	if !ok {
		return false
	}

	if message.Character == nil {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "You must select a character first.")
		return true
	}

	if strings.TrimSpace(text) == "" {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, channelUsage())
		return true
	}

	if channel == "admin" && (message.FromUser == nil || !message.FromUser.IsAdmin()) {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "Admin channel is restricted.")
		return true
	}

	out := messages.MessageResponse{
		Audience: messages.MessageAudienceGlobal,
		Type:     messages.MessageTypeDefault,
		Username: message.Character.Name,
		Message:  fmt.Sprintf("[%s] %s: %s", channelTitles[channel], message.Character.Name, strings.TrimSpace(text)),
		Channel:  channel,
	}
	game.SendMessage() <- out

	return true
}

// parseChannelInput resolves the channel and message text from raw input.
//
// It accepts both "chat <channel> <message>" and "<channel> <message>".
// A bare "chat" is recognized so the caller can show usage. The third return
// value is false when the first word is not a known channel, signalling that
// the input should not be intercepted.
func parseChannelInput(data string) (channel, text string, ok bool) {
	fields := strings.Fields(data)
	if len(fields) == 0 {
		return "", "", false
	}

	if strings.EqualFold(fields[0], "chat") {
		if len(fields) < 2 {
			return "", "", true
		}
		channel = strings.ToLower(fields[1])
		text = strings.Join(fields[2:], " ")
	} else {
		channel = strings.ToLower(fields[0])
		text = strings.Join(fields[1:], " ")
	}

	if _, known := channelTitles[channel]; !known {
		return "", "", false
	}

	return channel, text, true
}
