package def

import (
	"context"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/combat"
	"github.com/talesmud/talesmud/pkg/entities/items"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/entities/rooms"
	"github.com/talesmud/talesmud/pkg/instances"
	"github.com/talesmud/talesmud/pkg/service"
)

// NPCInstanceCtrl provides access to the NPC instance manager for scripts
type NPCInstanceCtrl interface {
	// GetInstance returns an NPC instance by ID
	GetInstance(id string) *npc.NPC
	// GetInstancesInRoom returns all alive NPC instances in a room
	GetInstancesInRoom(roomID string) []*npc.NPC
	// GetAllInstances returns all NPC instances
	GetAllInstances() []*npc.NPC
	// SpawnInstanceDirect spawns an NPC from a template (not via spawner)
	SpawnInstanceDirect(templateID, roomID string) (*npc.NPC, error)
	// KillInstance marks an NPC instance as dead
	KillInstance(id string) bool
	// DamageInstance applies damage and returns true if the NPC died
	DamageInstance(id string, amount int32) bool
	// HealInstance restores HP to an instance
	HealInstance(id string, amount int32) bool
	// MoveInstance moves an instance to a new room
	MoveInstance(id, roomID string) bool
	// UpdateInstance updates an instance using a callback
	UpdateInstance(id string, updater func(*npc.NPC)) bool
	// FindInstanceByNameInRoom finds an instance by name in a room
	FindInstanceByNameInRoom(roomID, name string) *npc.NPC
	// RemoveInstance drops an instance from the live set. The caller deletes any saved row.
	RemoveInstance(id string)
}

// CombatEngineCtrl provides access to the combat system for commands
type CombatEngineCtrl interface {
	// IsPlayerInCombat checks if a player is currently in combat
	IsPlayerInCombat(characterID string) bool
	// IsNPCInCombat checks if an NPC is currently in combat
	IsNPCInCombat(npcID string) bool
	// GetCombatInstance returns the combat instance a player is in
	GetCombatInstance(characterID string) *combat.CombatInstance
	// InitiateCombat starts combat between players and enemies
	InitiateCombat(roomID string, players []*characters.Character, enemies []*npc.NPC) *combat.CombatInstance
	// GetCombatInstanceByNPC returns the combat instance an NPC is in (if any)
	GetCombatInstanceByNPC(npcID string) *combat.CombatInstance
	// JoinCombat adds a character to an existing active combat instance
	JoinCombat(instance *combat.CombatInstance, character *characters.Character) bool
	// CombatGraceActive is true during the brief post-combat breath window
	CombatGraceActive(characterID string) bool
	// ClearCombatGrace removes the post-combat breath window (orphaned-fight recovery)
	ClearCombatGrace(characterID string)
	// ProcessPlayerAttack handles a player attacking a target in combat
	ProcessPlayerAttack(characterID, targetID string) (message string, combatEnded bool, endState combat.CombatState)
	// ProcessPlayerDefend handles a player defending
	ProcessPlayerDefend(characterID string) (message string, combatEnded bool, endState combat.CombatState)
	// ProcessPlayerFlee handles a player attempting to flee
	ProcessPlayerFlee(characterID string) (success bool, message string, combatEnded bool, endState combat.CombatState)
	// GetCombatStatus returns a formatted status string for the combat
	GetCombatStatus(characterID string) string
	// EndCombatForPlayer removes a player from combat (cleanup on disconnect, etc.)
	EndCombatForPlayer(characterID string)
	// QueuePlayerAction queues an action for a player's next auto-attack turn
	QueuePlayerAction(characterID string, action combat.CombatAction, targetID string)
	// SetAutoAttackTarget sets the persistent auto-attack target for a player
	SetAutoAttackTarget(characterID string, targetID string)
	// ProcessPlayerSkill handles a player using a skill in combat
	ProcessPlayerSkill(characterID, skillID, targetID string) (message string, combatEnded bool, endState combat.CombatState)
	// ApplyCombatDot applies a content-authored DoT during an active fight (Lua on-hit procs)
	ApplyCombatDot(attackerID, targetID, effectID, name string, damage int32, duration int) bool
	// HealCombatNPC heals a living enemy in the active fight and returns HP actually restored.
	HealCombatNPC(npcID string, amount int32) int32
	// SummonCombatAllies spawns enemy-template adds into the summoner's fight.
	// Returns how many this call spawned. Unknown template, a non-enemy, no fight,
	// or a full cap returns 0.
	SummonCombatAllies(summonerNPCID, templateID string, count int) int
	// BeginEngagement starts a fight the way attack does: swarm pack, onAggro once,
	// combatStart, and the party assist nudge. npcAggro posts the sight line and
	// uses the NPC as the aggressor. It does not apply the player's post-fight
	// breath window or the overlevel warning.
	BeginEngagement(roomID string, player *characters.Character, userID string, target *npc.NPC, npcAggro bool) *combat.CombatInstance
	// ApplyCombatEffect applies an existing buff or debuff skill id to a combatant in the fight.
	ApplyCombatEffect(targetID, effectID string) bool
	// QueuePlayerSkill queues a skill for a player's next turn
	QueuePlayerSkill(characterID, skillID, targetID string)
}

// QuestTrackerCtrl provides access to the quest tracker for event notifications
type QuestTrackerCtrl interface {
	OnNPCKilled(characterID, userID string, npc *npc.NPC)
	OnItemPickup(characterID, userID string, item *items.Item)
	OnRoomEnter(characterID, userID string, room *rooms.Room)
	OnDialogNode(characterID, userID, npcID, dialogID, nodeID string)
	OnTalkToNPC(characterID, userID string, npc *npc.NPC)
}

// OnlinePlayer describes a character currently attached to a live user session.
type OnlinePlayer struct {
	UserID        string
	CharacterID   string
	CharacterName string
	RoomID        string
	IsYou         bool
	LastSeen      time.Time
}

// PartyInviteTTL is how long a pending party invite stays valid.
const PartyInviteTTL = 45 * time.Second

// PartyInvite describes a pending invitation to join a party.
type PartyInvite struct {
	PartyID              string
	InviterUserID        string
	InviterCharacterID   string
	InviterCharacterName string
	TargetUserID         string
	TargetCharacterID    string
	TargetCharacterName  string
	CreatedAt            time.Time
}

// RoomInstanceCtrl is a gilded fantasy frame private copy of a small room graph.
type RoomInstanceCtrl interface {
	Enter(characterID, hubID, destID string) (clonedDestID string, err error)
	NoteLeave(characterID, fromRoomID, toRoomID string)
	DestroyCharacterInstance(characterID string)
	IsClone(roomID string) bool
	Generate(characterID string, playerLevel int32, spec instances.ProcSpec) (instances.ProcResult, error)
	Expire(now time.Time)
}

// ContextualQuestRequest describes one dynamically generated quest offer
// requested from a quest-giver NPC (Phase 6.4 wiring).
type ContextualQuestRequest struct {
	CharacterID   string
	UserID        string
	PlayerLevel   int
	ZoneID        string
	SourceNPCID   string
	SourceNPCName string
	Theme         string
}

// ContextualQuestProviderFunc generates one contextual quest for the request
// but must not persist it: the game stores the quest when the player accepts.
// Implementations are expected to degrade to deterministic content when no LLM
// provider is configured.
type ContextualQuestProviderFunc func(ctx context.Context, req ContextualQuestRequest) (*quests.Quest, error)

// QuestOfferStatus reports the outcome of scheduling a contextual quest offer.
type QuestOfferStatus int

const (
	// QuestOfferScheduled means generation was started (or completed inline).
	QuestOfferScheduled QuestOfferStatus = iota
	// QuestOfferDisabled means no provider is installed.
	QuestOfferDisabled
	// QuestOfferRateLimited means the character is still on cooldown.
	QuestOfferRateLimited
	// QuestOfferBusy means too many generations are already in flight.
	QuestOfferBusy
)

// NPC dialogue conversations reuse the authored conversation store with this
// sentinel DialogID. It lets DialogSelectCommand resolve numeric choices
// without touching the authored dialog graph.
const (
	NPCDialogueDialogID = "__llm__"

	// Conversation context keys for the pending actionable options. They live
	// in conversations.context (JSON already persisted), so no migration is
	// needed.
	NPCDialogueContextOptions        = "LLM_OPTIONS"
	NPCDialogueContextTemplate       = "LLM_NPC_TEMPLATE"
	NPCDialogueContextQuestion       = "LLM_QUESTION"
	NPCDialogueContextReply          = "LLM_REPLY"
	NPCDialogueContextAuthoredDialog = "LLM_AUTHORED_DIALOG"
)

// NPCDialogueOption is one suggested follow-up line returned by an LLM-driven
// NPC. Kind may be "flavor" (a conversational nudge), "end" (a polite
// conversation closer), or "quest:<questID>" (an available quest action).
// NodeID is filled by the runtime when a non-quest label maps to an authored
// dialog node; selecting it then continues that authored node.
type NPCDialogueOption struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	NodeID string `json:"nodeId,omitempty"`
}

// NPCDialogueResult is the canonical npc.dialogue payload. It is both the
// shape the prompt declares and the value the provider hands back to the game.
// RelationDelta is an optional model hint, always clamped to [-0.05,+0.05].
type NPCDialogueResult struct {
	Reply         string              `json:"reply"`
	Options       []NPCDialogueOption `json:"options,omitempty"`
	End           bool                `json:"end"`
	MemoryNotes   []string            `json:"memoryNotes,omitempty"`
	RelationDelta float64             `json:"relationDelta,omitempty"`
}

// NPCDialogueRequest describes one free-form question to an LLM-driven NPC
// (Phase 6.3 runtime cut). Persona/Voice/Temperature come from the NPC's
// LLMConfig, while Memory is an optional pre-rendered memory block; the
// installed provider normally refreshes it from NPCMemoriesService.
type NPCDialogueRequest struct {
	CharacterID   string
	UserID        string
	NPCID         string
	NPCTemplateID string
	NPCName       string
	CharacterName string
	PlayerLevel   int
	RoomID        string
	Question      string
	Persona       string
	Voice         string
	Temperature   float64
	Memory        string
	// CooldownSeconds is the per-(character, NPC) game-level cooldown. Zero
	// falls back to the NPC package default.
	CooldownSeconds int
	// Continuation marks a follow-up generated from an actionable LLM option.
	// It still uses the semaphore but skips the cooldown rejection so a player
	// can pick a flavor option immediately after the reply.
	Continuation bool
}

// NPCDialogueProviderFunc generates one in-character reply. Implementations
// must run off the command loop and are expected to respect ctx.
type NPCDialogueProviderFunc func(ctx context.Context, req NPCDialogueRequest) (NPCDialogueResult, error)

// NPCDialogueStatus reports the outcome of scheduling an NPC dialogue
// generation.
type NPCDialogueStatus int

const (
	// NPCDialogueScheduled means generation was started (or completed inline).
	NPCDialogueScheduled NPCDialogueStatus = iota
	// NPCDialogueDisabled means no dialogue provider is installed.
	NPCDialogueDisabled
	// NPCDialogueRateLimited means the character is still on cooldown for this NPC.
	NPCDialogueRateLimited
	// NPCDialogueBusy means too many generations are already in flight.
	NPCDialogueBusy
)

// PresenceState is a consistent view of the mutable presence fields of a
// user: IsOnline, LastSeen and LastCharacter. It is read under the presence
// lock of the user (Batch 23).
type PresenceState struct {
	IsOnline      bool
	LastSeen      time.Time
	LastCharacter string
}

// GameCtrl def
// interface for commands package to communicate back to game instance
type GameCtrl interface {
	// Used to pass messages as events inside the mud server, e.g. translate a command into other user messages etc.
	OnMessageReceived() chan interface{}
	// used to send replies/messages to users, origin or rooms, or global
	SendMessage() chan interface{}
	GetFacade() service.Facade
	// GetNPCInstanceManager returns the NPC instance controller
	GetNPCInstanceManager() NPCInstanceCtrl
	// GetCombatEngine returns the combat engine controller
	GetCombatEngine() CombatEngineCtrl
	// GetQuestTracker returns the quest tracker controller
	GetQuestTracker() QuestTrackerCtrl
	// GetRoomInstances returns the cellar/instance controller
	GetRoomInstances() RoomInstanceCtrl
	// InterruptRest stops a character from resting
	InterruptRest(char *characters.Character)
	// ConnectUserSession marks a user as attached to a live websocket session.
	ConnectUserSession(user *entities.User)
	// DisconnectUserSession removes a user's live session.
	DisconnectUserSession(userID string)
	// SetUserSessionCharacter attaches the selected character to a user's live session.
	SetUserSessionCharacter(user *entities.User, char *characters.Character)
	// PresenceOf returns the presence fields of a user under the presence
	// lock. The same user is shared by the websocket goroutine and the game
	// loop, so the mutable presence fields must be read through it (Batch 23).
	PresenceOf(user *entities.User) PresenceState
	// GetOnlinePlayers returns characters attached to live sessions.
	GetOnlinePlayers() []OnlinePlayer
	// FindOnlinePlayerByName finds a live player by active character name.
	FindOnlinePlayerByName(name string) (OnlinePlayer, bool)
	// GetRoomPlayers returns live players in a room.
	GetRoomPlayers(roomID, viewerCharacterID string) []OnlinePlayer
	// SetPartyInvite stores a pending invite for a live target character.
	SetPartyInvite(invite PartyInvite)
	// GetPartyInvite returns a pending invite for a target character.
	GetPartyInvite(targetCharacterID string) (PartyInvite, bool)
	// ClearPartyInvite removes a pending invite.
	ClearPartyInvite(targetCharacterID string)
	// RecordNeedGreedChoice records need/greed/pass for the open loot round.
	RecordNeedGreedChoice(characterID, userID, choice string) bool
	// HasActiveNeedGreed is true when the character is in the open loot round.
	HasActiveNeedGreed(characterID string) bool
	// NoteNeedGreedMemberLeft treats the character as Pass for open/queued rounds.
	NoteNeedGreedMemberLeft(characterID string)
	// SetPartyFollow marks followerID as following the party leader.
	SetPartyFollow(followerID, leaderID string)
	// ClearPartyFollow stops one character following. Reports the leader they were following.
	ClearPartyFollow(followerID string) (leaderID string, wasFollowing bool)
	// DropPartyFollowers stops everyone following leaderID and returns those character ids.
	DropPartyFollowers(leaderID string) []string
	// PartyFollowTarget returns the leader a character is following, if any.
	PartyFollowTarget(followerID string) (leaderID string, ok bool)
	// PullPartyFollowers relocates online followers after a normal exit walk.
	// Followers in fromRoomID take that step. Others chase along ordinary exits.
	// allow is false for teleports, portals, and private-instance crossings.
	PullPartyFollowers(leader *characters.Character, fromRoomID, destRoomID string, allow bool)
	// CatchUpPartyFollow walks a follower toward their leader, and walks anyone
	// following characterID toward that character. The follow flag is kept when
	// the path is blocked or someone is offline or in combat.
	CatchUpPartyFollow(characterID string)
	// NotePlayerEntered schedules same-room aggro-on-sight against aggressive NPCs already there.
	// Watches for any other room are cancelled and do not start the reaggro cooldown.
	NotePlayerEntered(characterID, roomID string)
	// NoteNPCAppeared schedules same-room aggro-on-sight for players already standing there.
	NoteNPCAppeared(npcID, roomID string)
	// Ready check methods for party coordination.
	StartReadyCheck(partyID string)
	SetReady(partyID, charID string, ready bool) bool
	GetReadyCheck(partyID string) (map[string]bool, time.Time, bool)
	ReadyCheckExpired(partyID string) bool
	ClearReadyCheck(partyID string)
	// RequestContextualQuestOffer generates and offers a dynamic quest to the
	// character. Generation runs off the command loop when the game is running.
	RequestContextualQuestOffer(req ContextualQuestRequest) QuestOfferStatus
	// PendingContextualQuest returns the last generated but not yet accepted
	// quest offer for a character.
	PendingContextualQuest(characterID string) (*quests.Quest, bool)
	// ClearPendingContextualQuest drops any pending generated quest offer.
	ClearPendingContextualQuest(characterID string)
	// RequestNPCDialogue generates and delivers an in-character reply from an
	// LLM-driven NPC. Generation runs off the command loop when the game is
	// running.
	RequestNPCDialogue(req NPCDialogueRequest) NPCDialogueStatus
	// NotifyDM posts a DM narration event (non-blocking, may be throttled/silenced).
	NotifyDM(e DMEvent)
	// DMNarratorEnabled reports whether a narrator is installed.
	DMNarratorEnabled() bool
}

// DMEvent is the typed event bus for the Dungeon Master narrator (Phase 6.5).
type DMEvent struct {
	Kind        string            `json:"kind"`
	RoomID      string            `json:"roomId,omitempty"`
	CharacterID string            `json:"characterId,omitempty"`
	UserID      string            `json:"userId,omitempty"`
	Facts       map[string]string `json:"facts,omitempty"`
}
