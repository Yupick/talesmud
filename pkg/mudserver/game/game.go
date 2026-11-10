package game

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/quests"

	"github.com/talesmud/talesmud/pkg/instances"
	c "github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	def "github.com/talesmud/talesmud/pkg/mudserver/game/def"
	m "github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/resources"
	"github.com/talesmud/talesmud/pkg/service"
)

// Game ... contains live game state
type Game struct {
	id    string
	title string

	// access to repository data
	Facade service.Facade

	SystemUser *entities.User

	// NPC instance manager for runtime NPC instances
	NPCManager *NPCInstanceManager

	// Combat controller for combat system
	CombatController *CombatController

	// Quest tracker for quest progress
	QuestTracker *QuestTracker

	// Private cellar copies (WoW-style, not a raid platform)
	RoomInstances *roomInstanceAdapter

	// Resources is the refilling balance store. Nil until the process attaches one.
	// An attached store with an empty catalog does not grant any uses.
	Resources *resources.Store

	// messages
	onMessageReceived chan interface{}
	sendMessage       chan interface{}

	OnUserJoined chan *m.UserJoined
	OnUserQuit   chan *m.UserQuit

	//OnAvatarJoinedRoom chan *AvatarJoinedRoom
	//OnAvatarLeftRoom   chan *AvatarLeftRoom

	Receivers []Receiver

	CommandProcessor *c.CommandProcessor
	RoomProcessor    *c.RoomProcessor

	Avatars map[string]*Avatar

	Sessions *sessionRegistry

	// presence owns the mutable presence fields (IsOnline, LastSeen,
	// LastCharacter) of every live *entities.User. The websocket goroutine,
	// the game loop and the connection-close goroutine share those users, so
	// every read and write of those fields goes through this lock (Batch 23).
	// It is a value, so the zero Game is usable.
	presence presenceRegistry

	// relocNotice is one login line per character moved by the instance sweep.
	// The sweep runs at process start, before anyone connects, so the map
	// only has to live until that character's next login in this process.
	relocMu     sync.Mutex
	relocNotice map[string]string

	// regenTick counts one-second regeneration passes on the server clock.
	// Each active pool is due when its interval divides this counter.
	regenTick uint64

	// aggroCh carries grace timers onto the command loop. The combat ticker
	// must not start a fight; it only requeues a send that the buffer dropped.
	aggroCh chan aggroFire
	aggro   aggroBook

	// jobs carries live-ops onto the command loop so they cannot race a player command.
	// loopOn is set only after Run starts that loop. Call runs inline until then.
	jobs   chan gameJob
	loopOn atomic.Bool

	// dmNarrator, when set, narrates combat/quest beats in prose. It runs off
	// the command loop and posts its line back through Call. Nil disables it.
	dmMu           sync.Mutex
	dmNarrator     DMNarratorFunc
	dmCooldown     map[string]time.Time
	dmSem          chan struct{}
	dmCooldownWin  time.Duration
	dmTimeout      time.Duration
	dmMaxPerMinute int
	dmMinuteMu     sync.Mutex
	dmMinuteEvents []time.Time
	dmEnabled      bool

	// Mode B (Phase 6.5): validated DM decisions may spawn an ambush or add
	// ambient prose. The provider and cooldown share dmDecisionMu; the global
	// budget has its own lock so it never nests with the provider lock.
	dmDecisionMu           sync.Mutex
	dmDecisionProvider     DMDecisionProvider
	dmDecisionCooldown     map[string]time.Time
	dmDecisionWindow       time.Duration
	dmDecisionTimeout      time.Duration
	dmDecisionMaxPerMinute int
	dmDecisionMinuteMu     sync.Mutex
	dmDecisionMinuteEvents []time.Time
	dmDecisionEnabled      bool
	// dmDecisionSpawn lets tests count ambush spawns without a real NPC
	// manager. When nil, production uses NPCManager.SpawnInstanceDirect.
	dmDecisionSpawn func(templateID, roomID string) error
	dmDecisionStats dmDecisionStats

	// questOfferMu guards the contextual quest offer state. The generator runs
	// off the command loop and posts results back through Call. A nil provider
	// disables the feature.
	questOfferMu       sync.Mutex
	questOfferProvider def.ContextualQuestProviderFunc
	questOfferCooldown map[string]time.Time
	questOfferPending  map[string]*quests.Quest
	questOfferSem      chan struct{}

	// npcDialogueMu guards the free-form NPC dialogue state (Phase 6.3). The
	// provider runs off the command loop and posts replies back through Call.
	// A nil provider disables the feature and callers fall back to authored
	// dialog.
	npcDialogueMu       sync.Mutex
	npcDialogueProvider def.NPCDialogueProviderFunc
	npcDialogueCooldown map[string]time.Time
	npcDialogueSem      chan struct{}

	//world *World
}

// New creates a new game instance
func New(facade service.Facade) *Game {
	g := &Game{

		title: "Lair of the Dragon",

		CommandProcessor: c.NewCommandProcessor(),
		RoomProcessor:    c.NewRoomProcessor(),

		// event channels
		onMessageReceived: make(chan interface{}, 20),
		sendMessage:       make(chan interface{}, 256),
		OnUserJoined:      make(chan *m.UserJoined, 20),
		OnUserQuit:        make(chan *m.UserQuit, 20),

		// game update listeners
		//	Receivers: make([]Receiver, 0, 10),

		Avatars:  make(map[string]*Avatar),
		Sessions: newSessionRegistry(),

		Facade: facade,
	}
	g.aggroCh = make(chan aggroFire, 64)
	g.aggro.pending = map[string]*aggroWatch{}
	g.aggro.cooldown = map[string]time.Time{}
	g.aggro.ch = g.aggroCh
	g.jobs = make(chan gameJob, 32)
	g.dmCooldown = map[string]time.Time{}
	g.dmSem = make(chan struct{}, dmMaxConcurrent)
	g.dmCooldownWin = dmCooldownWindowDefault
	g.dmTimeout = dmTimeoutDefault
	g.dmMaxPerMinute = dmMaxPerMinuteDefault
	if v := strings.TrimSpace(os.Getenv("LLM_DM_COOLDOWN_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			g.dmCooldownWin = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("LLM_DM_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			g.dmTimeout = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("LLM_DM_MAX_PER_MIN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			g.dmMaxPerMinute = n
		}
	}
	g.dmEnabled = true
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_DM_ENABLED"))); v != "" {
		switch v {
		case "0", "false", "no", "off":
			g.dmEnabled = false
		default:
			g.dmEnabled = true
		}
	}
	g.dmDecisionCooldown = map[string]time.Time{}
	g.dmDecisionWindow = dmDecisionCooldownDefault
	g.dmDecisionTimeout = dmDecisionTimeoutDefault
	g.dmDecisionMaxPerMinute = dmDecisionMaxPerMinute
	if v := strings.TrimSpace(os.Getenv("LLM_DM_DECISION_COOLDOWN_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			g.dmDecisionWindow = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("LLM_DM_DECISION_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			g.dmDecisionTimeout = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("LLM_DM_DECISION_MAX_PER_MIN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			g.dmDecisionMaxPerMinute = n
		}
	}
	g.dmDecisionEnabled = dmDecisionKillSwitch()
	g.questOfferCooldown = map[string]time.Time{}
	g.questOfferPending = map[string]*quests.Quest{}
	g.questOfferSem = make(chan struct{}, questOfferMaxConcurrent)
	g.npcDialogueCooldown = map[string]time.Time{}
	g.npcDialogueSem = make(chan struct{}, npcDialogueMaxConcurrent)

	// Initialize NPC instance manager
	g.NPCManager = NewNPCInstanceManager(facade)

	// Initialize Combat controller
	g.CombatController = NewCombatController(g)

	// Initialize Quest tracker
	g.QuestTracker = NewQuestTracker(facade, g)

	g.RoomInstances = &roomInstanceAdapter{game: g, mgr: instances.NewManager()}

	return g
}

// GetRoomInstances returns the cellar instance controller.
func (g *Game) GetRoomInstances() def.RoomInstanceCtrl {
	return g.RoomInstances
}

// Subscribe ... sub
//func (g *Game) Subscribe(receiver Receiver) {
//	g.Receivers = append(g.Receivers, receiver)
//}

// Receiver ... rec
type Receiver interface {
	OnMessage(message interface{})
}

// Unsubscribe ...
func (g *Game) Unsubscribe(receiver *Receiver) {
	//TODO:
	//game.Receivers = delete(game.Receivers, receiver)
}

/*
// SendMessage ...
func (g *Game) SendMessage(msg interface{}) {
	// broeadcast message to all game listeners (currently only websocket server)
	for _, receiver := range g.Receivers {
		receiver.OnMessage(msg)
	}
}*/

// SendMessage ...
func (g *Game) SendMessage() chan interface{} {
	return g.sendMessage
}

// DispatchCommand runs one command against the current character, synchronously.
func (g *Game) DispatchCommand(user *entities.User, text string) {
	if g == nil || user == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	msg := m.NewMessage(user, text)
	msg.NoticeGen = m.BeginNotice(user.ID)
	defer m.EndNotice(user.ID)
	g.attachCharacterToMessage(msg)
	if !g.CommandProcessor.Process(g, msg) {
		g.RoomProcessor.Process(g, msg)
	}
}

// OnMessageReceived returns onMessageReceived channel
func (g *Game) OnMessageReceived() chan interface{} {
	return g.onMessageReceived
}

// GetFacade ...
func (g *Game) GetFacade() service.Facade {
	return g.Facade
}

// GetNPCInstanceManager returns the NPC instance manager
func (g *Game) GetNPCInstanceManager() def.NPCInstanceCtrl {
	return g.NPCManager
}

// GetCombatEngine returns the combat engine controller
func (g *Game) GetCombatEngine() def.CombatEngineCtrl {
	return g.CombatController
}

// GetQuestTracker returns the quest tracker controller
func (g *Game) GetQuestTracker() def.QuestTrackerCtrl {
	return g.QuestTracker
}

const roomUpdateInterval = 10
const npcUpdateInterval = 10
const spawnerUpdateInterval = 5
const combatUpdateInterval = 1 // Combat pacing tick (C1 authored beats + 5s player window)
const regenUpdateInterval = 1  // one-second clock; each regen pool has its own interval

func (g *Game) handleGameUpdates() {

	roomTicker := time.NewTicker(roomUpdateInterval * time.Second)
	npcTicker := time.NewTicker(npcUpdateInterval * time.Second)
	spawnerTicker := time.NewTicker(spawnerUpdateInterval * time.Second)
	combatTicker := time.NewTicker(combatUpdateInterval * time.Second)
	regenTicker := time.NewTicker(regenUpdateInterval * time.Second)

	for {
		select {
		case <-roomTicker.C:
			g.handleRoomUpdates()
		case <-npcTicker.C:
			_ = g.Call(func() { g.handleNPCUpdates() })
		case <-spawnerTicker.C:
			g.handleSpawnerUpdates()
		case <-combatTicker.C:
			g.handleCombatUpdates()
		case <-regenTicker.C:
			g.handleRegenerationUpdates()
		}
	}
}

// handleCombatUpdates processes combat tick (turn timeouts, NPC actions)
func (g *Game) handleCombatUpdates() {
	g.nudgeDueAggro()
	if g.CombatController != nil {
		g.CombatController.Update()
	}
}

// Run main game loop
func (g *Game) Run() {

	// Initialize NPC instance manager (spawns initial NPCs from spawners)
	if err := g.NPCManager.Initialize(); err != nil {
		log.WithError(err).Error("Failed to initialize NPC instance manager")
	}

	go g.handleGameUpdates()

	g.loopOn.Store(true)
	go func() {
		for {
			select {
			case job := <-g.jobs:
				g.runJob(job)

			case userJoined := <-g.OnUserJoined:
				log.Debug("UserJoined channel received")
				g.handleUserJoined(userJoined.User)

			case userQuit := <-g.OnUserQuit:
				log.WithFields(log.Fields{
					"userId":   userQuit.User.ID,
					"nickname": userQuit.User.Nickname,
				}).Debug("UserQuit channel received")
				g.handleUserQuit(userQuit.User)

			case fire := <-g.aggroCh:
				g.fireAggro(fire.key, fire.gen)

			case msg := <-g.onMessageReceived:
				g.processIncoming(msg)
			}
		}
	}()
}

// processIncoming dispatches one inbound message on the game-loop goroutine.
//
// It must only run on the goroutine that consumes onMessageReceived (Run's
// loop, or code already executing on it such as handleUserJoined). Sending a
// message to onMessageReceived from that goroutine is a self-deadlock: the
// loop is the channel's only consumer, so a full buffer blocks the very
// goroutine that would drain it (QA B19.3). Synthesized input such as "lc"
// and "selectcharacter" is therefore dispatched inline instead of round-tripping
// through the queue — the command still executes exactly once, in the same
// execution context, immediately after the join that produced it.
func (g *Game) processIncoming(msg interface{}) {
	switch message := msg.(type) {
	case *m.Message:
		// attach current character if a user is set
		g.attachCharacterToMessage(message)

		// only broadcast if global commandprocessor didnt process it
		if !g.CommandProcessor.Process(g, message) {
			// check room commands
			if !g.RoomProcessor.Process(g, message) && !c.BareSkill(g, message) {
				if c.LLMDialogFreeText(g, message) {
					return
				}
				// generic messages will be converted to plain OutgoingMessages (type message)
				// and send to the room audience including the origin nickname or charactername
				g.handleDefaultMessage(message)
			}
		}
	}
}
