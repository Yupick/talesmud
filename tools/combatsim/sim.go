package main

// sim.go runs combat through the REAL combat engine
// (pkg/mudserver/game/combat). Only the player action policy below is tool code;
// damage, hit rolls, crits, level-gap modifiers, class balance, class kit
// effects and NPC AI all come from the engine.
//
// Policy is documented in docs/qa/BALANCE_ANALYSIS.md (section B18). It is a
// competent-but-not-optimal heuristic: heal when hurt, use one-off setup
// buttons early, then the best available offensive button, otherwise autoattack.

import (
	"sort"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities/characters"
	entcombat "github.com/talesmud/talesmud/pkg/entities/combat"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/skills"
	combat "github.com/talesmud/talesmud/pkg/mudserver/game/combat"
	"github.com/talesmud/talesmud/pkg/mudserver/game/combat/simutil"
)

const maxSimRounds = 200

// runStats is the per-run outcome.
type runStats struct {
	state     entcombat.CombatState
	rounds    int
	dealtHP   int32 // effective damage to the enemy (enemyMax - enemyEnd)
	takenNet  int32 // net player HP lost (playerMax - playerEnd)
	directDmg int64 // sum of action damage reported by the engine
	healed    int64 // sum of heals reported by the engine
	playerMax int32
	playerEnd int32
	enemyMax  int32
	enemyEnd  int32
}

// simulate runs one fight between one player and one enemy.
func simulate(player *characters.Character, enemy *npc.NPC) runStats {
	engine := combat.NewEngine(combat.NewManager(), nil)
	instance := engine.InitiateCombat("combatsim", []*characters.Character{player}, []*npc.NPC{enemy})

	var st runStats
	st.playerMax = totalHP(instance.Players)
	st.enemyMax = totalHP(instance.Enemies)

	for round := 0; round < maxSimRounds; round++ {
		current := instance.GetCurrentTurnCombatant()
		if current == nil {
			break
		}

		if current.Type == entcombat.CombatantTypeNPC {
			action, targetID := engine.GetNPCAIAction(instance, current, nil)
			switch action {
			case entcombat.CombatActionAttack:
				step := engine.StepNPCAttack(instance, current.ID, targetID)
				st.directDmg += int64(step.Attack.Damage)
			case entcombat.CombatActionDefend:
				engine.ProcessDefend(instance, current.ID)
			}
		} else {
			playerTurn(engine, instance, current.ID, &st)
		}

		if endState := engine.CheckCombatEnd(instance); endState != entcombat.CombatStateActive {
			st.state = endState
			st.rounds = instance.Round
			st.playerEnd = totalHP(instance.Players)
			st.enemyEnd = totalHP(instance.Enemies)
			st.dealtHP = st.enemyMax - st.enemyEnd
			st.takenNet = st.playerMax - st.playerEnd
			return st
		}

		engine.NextTurn(instance)
	}

	st.state = entcombat.CombatStateTimeout
	st.rounds = maxSimRounds
	st.playerEnd = totalHP(instance.Players)
	st.enemyEnd = totalHP(instance.Enemies)
	st.dealtHP = st.enemyMax - st.enemyEnd
	st.takenNet = st.playerMax - st.playerEnd
	return st
}

// playerTurn runs the tool's player policy for one turn.
func playerTurn(engine *combat.Engine, instance *entcombat.CombatInstance, playerID string, st *runStats) {
	target := firstLivingEnemy(instance)
	if target == nil {
		return
	}
	player := instance.GetCombatantByID(playerID)
	if player == nil || !player.IsAlive {
		return
	}

	// 1. Emergency / sustainability heals.
	if pct(player) <= 0.50 {
		if sk := readySkill(player, "mend"); sk != nil {
			useSkill(engine, instance, playerID, sk, playerID, st)
			return
		}
	}
	if pct(player) <= 0.70 && !hasEffect(player, "renew") {
		if sk := readySkill(player, "renew"); sk != nil {
			useSkill(engine, instance, playerID, sk, playerID, st)
			return
		}
	}

	// 2. Once-per-fight setup, in a stable order.
	setupOrder := []string{"brace", "glyph", "guard", "stand", "rig", "bolt", "inscribe"}
	for _, kit := range setupOrder {
		if !setupWanted(instance, player, kit) {
			continue
		}
		if sk := readySkill(player, kit); sk != nil {
			tgt := target.ID
			if kit == "brace" || kit == "glyph" || kit == "guard" || kit == "stand" {
				tgt = playerID
			}
			if kit == "rig" || kit == "bolt" {
				tgt = target.ID
			}
			if useSkill(engine, instance, playerID, sk, tgt, st) {
				return
			}
		}
	}

	// 3. Overload the rig once it is up.
	if instance.Rig != nil && instance.Rig.RoundsLeft > 0 && instance.Rig.OwnerID == playerID {
		if sk := readySkill(player, "overload"); sk != nil {
			if useSkill(engine, instance, playerID, sk, "", st) {
				return
			}
		}
	}

	// 4. Best available offensive button.
	if sk := bestOffense(player); sk != nil {
		if useSkill(engine, instance, playerID, sk, chooseTarget(instance, player, sk, target.ID), st) {
			return
		}
	}

	// 5. Autoattack.
	res := engine.ProcessAttack(instance, playerID, target.ID)
	st.directDmg += int64(res.Damage)
}

// useSkill fires a skill and returns true when it was spent.
func useSkill(engine *combat.Engine, instance *entcombat.CombatInstance, playerID string, sk *skills.Skill, targetID string, st *runStats) bool {
	res := engine.ProcessSkill(instance, playerID, sk.Entity.ID, targetID)
	if !res.Success {
		return false
	}
	st.directDmg += int64(res.TotalDamage)
	st.healed += int64(res.TotalHeal)
	// A "keeps swing" button still lets the basic attack land (game_combat.go).
	if res.KeepsSwing {
		if again := instance.GetCombatantByID(playerID); again != nil && again.IsAlive && !again.HasFled {
			t := firstLivingEnemy(instance)
			if t != nil {
				ar := engine.ProcessAttack(instance, playerID, t.ID)
				st.directDmg += int64(ar.Damage)
			}
		}
	}
	return true
}

// setupWanted gates once-per-fight buttons that have no useful early use.
func setupWanted(instance *entcombat.CombatInstance, player *entcombat.CombatantRef, kit string) bool {
	if player.KitSpent != nil && player.KitSpent[skillIDFor(player, kit)] {
		return false
	}
	switch kit {
	case "rig":
		return player.RigLeft > 0 && instance.Rig == nil
	case "bolt":
		return player.BoltLeft > 0
	case "inscribe":
		enemy := firstLivingEnemy(instance)
		return enemy != nil && !hasEffect(enemy, "inscribe")
	case "brace":
		return player.BraceLeft <= 0
	case "glyph":
		return player.GlyphCut <= 0
	case "guard":
		return player.GuardRounds <= 0
	case "stand":
		return player.StandRounds <= 0
	}
	return true
}

// chooseTarget sends ally/self skills at the player and everything else at the
// current enemy.
func chooseTarget(instance *entcombat.CombatInstance, player *entcombat.CombatantRef, sk *skills.Skill, enemyID string) string {
	switch sk.Target {
	case skills.TargetSelf:
		return player.ID
	case skills.TargetAlly:
		return player.ID
	default:
		return enemyID
	}
}

// readySkill finds the first equipped, off-cooldown, affordable skill with the
// given kit id.
func readySkill(player *entcombat.CombatantRef, kit string) *skills.Skill {
	for _, id := range player.EquippedSkills {
		sk := skills.SkillByID(id)
		if sk == nil || sk.Kit != kit {
			continue
		}
		if player.SkillCooldowns != nil && player.SkillCooldowns[id] > 0 {
			continue
		}
		if sk.ManaCost > 0 && player.CurrentMana < sk.ManaCost {
			continue
		}
		return sk
	}
	return nil
}

// bestOffense ranks damage kits by, in order: nick (multi-swing for rogues),
// sear, slam. This mirrors the intent that the signature nuke/slam is used on
// cooldown while rogues lean on their extra-swing kit.
func bestOffense(player *entcombat.CombatantRef) *skills.Skill {
	for _, kit := range []string{"nick", "sear", "slam"} {
		if sk := readySkill(player, kit); sk != nil {
			return sk
		}
	}
	return nil
}

func skillIDFor(player *entcombat.CombatantRef, kit string) string {
	for _, id := range player.EquippedSkills {
		if sk := skills.SkillByID(id); sk != nil && sk.Kit == kit {
			return id
		}
	}
	return ""
}

func firstLivingEnemy(instance *entcombat.CombatInstance) *entcombat.CombatantRef {
	enemies := instance.GetLivingEnemies()
	if len(enemies) == 0 {
		return nil
	}
	// Deterministic: lowest HP first would change with rolls; keep order stable.
	sort.SliceStable(enemies, func(i, j int) bool { return enemies[i].ID < enemies[j].ID })
	return enemies[0]
}

func hasEffect(c *entcombat.CombatantRef, skillIDFragment string) bool {
	if c == nil {
		return false
	}
	for _, se := range c.StatusEffects {
		if se.Duration > 0 && strings.Contains(strings.ToLower(se.SkillID), strings.ToLower(skillIDFragment)) {
			return true
		}
	}
	return false
}

func pct(c *entcombat.CombatantRef) float64 {
	if c == nil || c.MaxHP <= 0 {
		return 1
	}
	return float64(c.CurrentHP) / float64(c.MaxHP)
}

func totalHP(list []entcombat.CombatantRef) int32 {
	var total int32
	for _, c := range list {
		if c.CurrentHP > 0 {
			total += c.CurrentHP
		}
	}
	return total
}

// aggregate folds many runs into summary metrics.
type aggregate struct {
	class      string
	tier       string
	level      int32
	iters      int
	wins       int
	losses     int
	timeouts   int
	sumRounds  int64
	sumDealt   int64
	sumTaken   int64
	sumDirect  int64
	sumHealed  int64
	playerHPMax int32
	enemyHPMax  int32
}

func (a *aggregate) add(st runStats) {
	a.iters++
	switch st.state {
	case entcombat.CombatStateVictory:
		a.wins++
	case entcombat.CombatStateDefeat:
		a.losses++
	default:
		a.timeouts++
	}
	a.sumRounds += int64(st.rounds)
	a.sumDealt += int64(st.dealtHP)
	a.sumTaken += int64(st.takenNet)
	a.sumDirect += st.directDmg
	a.sumHealed += st.healed
	a.playerHPMax = st.playerMax
	a.enemyHPMax = st.enemyMax
}

func (a *aggregate) winRate() float64 {
	if a.iters == 0 {
		return 0
	}
	return float64(a.wins) / float64(a.iters)
}

// lossRate is the fraction of runs that ended in a player defeat.
func (a *aggregate) lossRate() float64 {
	if a.iters == 0 {
		return 0
	}
	return float64(a.losses) / float64(a.iters)
}

// timeoutRate is the fraction of runs that hit maxSimRounds with no victor.
func (a *aggregate) timeoutRate() float64 {
	if a.iters == 0 {
		return 0
	}
	return float64(a.timeouts) / float64(a.iters)
}

func (a *aggregate) avgRounds() float64 {
	if a.iters == 0 {
		return 0
	}
	return float64(a.sumRounds) / float64(a.iters)
}

// dps is effective damage output per combat round.
func (a *aggregate) dps() float64 {
	if a.sumRounds == 0 {
		return 0
	}
	return float64(a.sumDealt) / float64(a.sumRounds)
}

// hps is net player HP lost per combat round.
func (a *aggregate) hps() float64 {
	if a.sumRounds == 0 {
		return 0
	}
	return float64(a.sumTaken) / float64(a.sumRounds)
}

// runMatchup runs iterations fights of one class/level/tier.
func runMatchup(entry rosterEntry, tier string, difficulty string, gear string, iterations int) aggregate {
	agg := aggregate{class: entry.classID, tier: tier, level: entry.level}
	for i := 0; i < iterations; i++ {
		player := buildCharacter(entry.tpl, entry.level, gear)
		enemy := simutil.CreateScaledEnemy(tier, entry.level, difficulty)
		agg.add(simulate(player, enemy))
	}
	return agg
}
