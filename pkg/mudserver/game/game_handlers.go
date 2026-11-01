package game

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// handleDefaultMessage implements implicit "say" behavior.
// When player input doesn't match any command, treat it as room speech.
// This allows players to type "hello everyone" instead of "say hello everyone".
func (game *Game) handleDefaultMessage(message *messages.Message) {

	user := ""

	if message.FromUser != nil {
		user = message.FromUser.Nickname
		if message.Character != nil {
			user = message.Character.Name
		}
	}

	out := messages.NewRoomBasedMessage(user, message.Data)

	if message.Character != nil {
		out.AudienceID = message.Character.CurrentRoomID
	}

	game.SendMessage() <- out
}

func (game *Game) handleUserQuit(user *entities.User) {

	// The active character is read through the presence lock: the websocket
	// goroutine can select a character while the game loop quits the user
	// (Batch 23).
	lastCharacter := game.PresenceOf(user).LastCharacter

	log.WithFields(log.Fields{
		"userId":      user.ID,
		"nickname":    user.Nickname,
		"characterId": lastCharacter,
	}).Info("UserQuit")

	game.DisconnectUserSession(user.ID)

	if lastCharacter == "" {
		return
	}
	character, err := game.Facade.CharactersService().FindByID(lastCharacter)
	if err != nil || character == nil {
		log.WithField("characterID", lastCharacter).WithError(err).Warn("Could not load character during user quit")
		return
	}
	room, err := game.Facade.RoomsService().FindByID(character.CurrentRoomID)
	if err != nil || room == nil {
		log.WithField("roomID", character.CurrentRoomID).WithError(err).Warn("Could not load room during user quit")
		return
	}

	//TOOD: move update to queue
	room.RemoveCharacter(character.ID)
	game.Facade.RoomsService().Update(room.ID, room)

	game.SendMessage() <- messages.CharacterLeftRoom{
		MessageResponse: messages.MessageResponse{
			Audience:   messages.MessageAudienceRoomWithoutOrigin,
			OriginID:   character.ID,
			AudienceID: character.CurrentRoomID,
			Message:    character.Name + " left.",
		},
	}
	game.SendMessage() <- messages.NewRoomPresenceMessage(room, game)
}

// Find the matching character for the user where the message originated
//
// The active character is read through the presence lock: LastCharacter is
// written by the game loop and read here, which can be the websocket
// goroutine (Batch 23).
func (game *Game) attachCharacterToMessage(msg *messages.Message) {

	if msg.Character != nil {
		return
	}

	// could be a processed message that got the user removed
	if msg.FromUser == nil {
		return
	}
	lastCharacter := game.PresenceOf(msg.FromUser).LastCharacter
	if lastCharacter == "" {
		return
	}

	if character, err := game.Facade.CharactersService().FindByID(lastCharacter); err == nil {
		character.NormalizeAttributeShorts()
		msg.Character = character
	} else {
		log.Error("Couldt not load character for user")
	}
}

func (game *Game) handleUserJoined(user *entities.User) {
	// The active character is read through the presence lock: the websocket
	// goroutine can select a character while the game loop joins the user
	// (Batch 23).
	presence := game.PresenceOf(user)

	log.WithFields(log.Fields{
		"userId":      user.ID,
		"nickname":    user.Nickname,
		"characterId": presence.LastCharacter,
	}).Info("UserJoined")
	game.ConnectUserSession(user)

	// get active character for user
	if presence.LastCharacter == "" {

		if chars, err := game.Facade.CharactersService().FindAllForUser(user.ID); err == nil {

			// take first character for now
			// TODO: let the player choose?
			if len(chars) > 0 {
				//TODO: send updates via message queue?
				game.ApplyPresence(user, func(u *entities.User) bool {
					u.LastCharacter = chars[0].ID
					u.LastSeen = time.Now()
					u.IsOnline = true
					return true
				})
				presence = game.PresenceOf(user)
			}
		} else {
			// player has no character yet, respnd with createCharacter Message
			game.SendMessage() <- messages.NewCreateCharacterMessage(user.ID)
			return
		}
	}

	if character, err := game.Facade.CharactersService().FindByID(presence.LastCharacter); err != nil {

		log.WithField("user", user.Name).Error("Could not select character for user")
		// player character may be broken, let the user create a new one
		//game.SendMessage(messages.NewCreateCharacterMessage(user.ID))
		// send list characters command
		//
		// Dispatched inline instead of game.onMessageReceived <- ...: this
		// handler runs on the game loop, which is the only consumer of that
		// channel. Sending to it from here filled the buffer until the loop
		// blocked on a send only it could service (permanent wedge under
		// >=200 concurrent logins, QA B19.3). Inline never drops the message —
		// "lc" is the login fallback for a user without a usable character.
		game.processIncoming(messages.NewMessage(user, "lc"))
	} else {

		// select the active character inline, for the same self-deadlock
		// reason as the "lc" branch above (QA B19.3, game_handlers.go:133).
		game.processIncoming(messages.NewMessage(user, "selectcharacter "+character.Name))
	}
}
