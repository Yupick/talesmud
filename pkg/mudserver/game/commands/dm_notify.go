package commands

import (
	"strconv"

	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/leveling"
)

// notifyQuestCompletedDM posts a private quest-completed beat to the Dungeon
// Master. It is best-effort: the narrator itself decides whether to throttle
// or stay silent.
func notifyQuestCompletedDM(game def.GameCtrl, char *characters.Character, quest *quests.Quest) {
	if game == nil || char == nil || quest == nil {
		return
	}
	game.NotifyDM(def.DMEvent{
		Kind:        "questCompleted",
		RoomID:      char.CurrentRoomID,
		CharacterID: char.ID,
		UserID:      char.BelongsUserID,
		Facts: map[string]string{
			"questName": quest.Name,
		},
	})
}

// notifyLevelUpDM posts a private level-up beat to the Dungeon Master. The
// online session is used for the room/character hints when available; the
// narration still works when the player is offline (User audience only).
func notifyLevelUpDM(game def.GameCtrl, userID string, levelUp *leveling.LevelUpResult) {
	if game == nil || userID == "" || levelUp == nil || levelUp.LevelsGained <= 0 {
		return
	}
	roomID, characterID := "", ""
	for _, player := range game.GetOnlinePlayers() {
		if player.UserID == userID {
			roomID, characterID = player.RoomID, player.CharacterID
			break
		}
	}
	game.NotifyDM(def.DMEvent{
		Kind:        "levelUp",
		RoomID:      roomID,
		CharacterID: characterID,
		UserID:      userID,
		Facts: map[string]string{
			"level": strconv.Itoa(int(levelUp.NewLevel)),
		},
	})
}
