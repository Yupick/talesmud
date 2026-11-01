package main

// Tests for the pure harness logic in combatsim: CLI parsing, the once-per-fight
// policy gates, and the aggregate metrics. The engine itself is exercised by the
// sim run (and by pkg/mudserver/game/combat tests), so these tests stay on tool
// code only and need no class catalog on disk.

import (
	"strings"
	"testing"
	"time"

	entcombat "github.com/talesmud/talesmud/pkg/entities/combat"
)

func TestParseList(t *testing.T) {
	got := parseList(" warrior , rogue,,")
	want := []string{"warrior", "rogue"}
	if len(got) != len(want) {
		t.Fatalf("parseList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if out := parseList(""); out != nil {
		t.Fatalf("parseList(\"\") = %v, want nil", out)
	}
}

func TestParseInts(t *testing.T) {
	got := parseInts("1, 10,0,-3,25,abc")
	want := []int32{1, 10, 25}
	if len(got) != len(want) {
		t.Fatalf("parseInts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseInts[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestDifficultyFor(t *testing.T) {
	cases := map[string]string{
		"trash": "easy",
		"Trash": "easy",
		"elite": "hard",
		"boss":  "boss",
		"normal": "normal",
		"":      "normal",
		"weird": "normal",
	}
	for tier, want := range cases {
		if got := difficultyFor(tier); got != want {
			t.Errorf("difficultyFor(%q) = %q, want %q", tier, got, want)
		}
	}
}

// The boss cell is where timeouts hide: a0% win rate must be split into
// "died fast" vs "hit the 200-round cap" for the report to be honest.
func TestAggregateWinLossTimeoutSplit(t *testing.T) {
	agg := aggregate{class: "warrior", tier: "boss", level: 50}
	agg.add(runStats{state: entcombat.CombatStateVictory, rounds: 20, dealtHP: 100, takenNet: 10, playerMax: 50, playerEnd: 40, enemyMax: 120, enemyEnd: 0})
	agg.add(runStats{state: entcombat.CombatStateDefeat, rounds: 12, dealtHP: 60, takenNet: 50, playerMax: 50, playerEnd: 0, enemyMax: 120, enemyEnd: 60})
	agg.add(runStats{state: entcombat.CombatStateTimeout, rounds: maxSimRounds, dealtHP: 90, takenNet: 20, playerMax: 50, playerEnd: 30, enemyMax: 120, enemyEnd: 30})

	if agg.winRate() != 1.0/3.0 {
		t.Errorf("winRate = %v, want 1/3", agg.winRate())
	}
	if agg.lossRate() != 1.0/3.0 {
		t.Errorf("lossRate = %v, want 1/3", agg.lossRate())
	}
	if agg.timeoutRate() != 1.0/3.0 {
		t.Errorf("timeoutRate = %v, want 1/3", agg.timeoutRate())
	}
	if got := agg.avgRounds(); got != float64(20+12+maxSimRounds)/3 {
		t.Errorf("avgRounds = %v", got)
	}
	// dps = sum(dealt)/sum(rounds), hps = sum(taken)/sum(rounds).
	sumRounds := float64(20 + 12 + maxSimRounds)
	if got := agg.dps(); got != 250.0/sumRounds {
		t.Errorf("dps = %v, want %v", got, 250.0/sumRounds)
	}
	if got := agg.hps(); got != 80.0/sumRounds {
		t.Errorf("hps = %v, want %v", got, 80.0/sumRounds)
	}
}

func TestAggregateEmptyIsZero(t *testing.T) {
	var agg aggregate
	if agg.winRate() != 0 || agg.lossRate() != 0 || agg.timeoutRate() != 0 {
		t.Fatalf("empty rates should be 0: %v %v %v", agg.winRate(), agg.lossRate(), agg.timeoutRate())
	}
	if agg.avgRounds() != 0 || agg.dps() != 0 || agg.hps() != 0 {
		t.Fatalf("empty averages should be 0: %v %v %v", agg.avgRounds(), agg.dps(), agg.hps())
	}
}

// Once-per-fight setup buttons are gated so a spent kit never eats a turn.
func TestSetupWantedGates(t *testing.T) {
	player := &entcombat.CombatantRef{ID: "p1"}
	instance := &entcombat.CombatInstance{}

	// Fresh rig: wanted while no rig is on the field and charges remain.
	player.RigLeft = 1
	if !setupWanted(instance, player, "rig") {
		t.Fatal("rig should be wanted with RigLeft=1 and no rig on field")
	}
	instance.Rig = &entcombat.RigTurret{OwnerID: "p1", RoundsLeft: 2}
	if setupWanted(instance, player, "rig") {
		t.Fatal("rig should not be re-cast while one is live")
	}
	instance.Rig = nil
	player.RigLeft = 0
	if setupWanted(instance, player, "rig") {
		t.Fatal("rig with no charges must not be wanted")
	}

	// Brace is wanted until it is armed (BraceLeft > 0).
	brace := &entcombat.CombatantRef{ID: "p2"}
	if !setupWanted(instance, brace, "brace") {
		t.Fatal("brace should be wanted while unarmed")
	}
	brace.BraceLeft = 1
	if setupWanted(instance, brace, "brace") {
		t.Fatal("brace must not be re-cast while armed")
	}

	// A spent once-per-fight skill is never wanted again.
	spent := &entcombat.CombatantRef{
		ID:       "p3",
		BoltLeft: 1,
		KitSpent: map[string]bool{},
	}
	if !setupWanted(instance, spent, "bolt") {
		t.Fatal("bolt should be wanted with charges left")
	}
	// With no catalog loaded skillIDFor resolves nothing; a KitSpent entry for
	// a different skill id must not block the bolt gate.
	spent.KitSpent["some_other_skill"] = true
	if !setupWanted(instance, spent, "bolt") {
		t.Fatal("unrelated KitSpent entry must not block a different kit")
	}
}

func TestPctAndTotalHP(t *testing.T) {
	c := &entcombat.CombatantRef{CurrentHP: 30, MaxHP: 50}
	if got := pct(c); got != 0.6 {
		t.Errorf("pct = %v, want 0.6", got)
	}
	if got := pct(nil); got != 1 {
		t.Errorf("pct(nil) = %v, want 1", got)
	}
	if got := pct(&entcombat.CombatantRef{CurrentHP: 5, MaxHP: 0}); got != 1 {
		t.Errorf("pct(MaxHP=0) = %v, want 1", got)
	}

	list := []entcombat.CombatantRef{
		{CurrentHP: 10}, {CurrentHP: 0}, {CurrentHP: 7},
	}
	if got := totalHP(list); got != 17 {
		t.Errorf("totalHP = %d, want 17 (dead bodies contribute 0)", got)
	}
}

func TestRenderMarkdownHasSplitColumns(t *testing.T) {
	agg := aggregate{class: "warrior", tier: "boss", level: 50}
	agg.add(runStats{state: entcombat.CombatStateDefeat, rounds: 10})
	agg.add(runStats{state: entcombat.CombatStateTimeout, rounds: maxSimRounds})
	md := renderMarkdown([]aggregate{agg}, 2, 1500*time.Millisecond)
	for _, want := range []string{
		"| Class | Level | Tier | Win% | Loss% | Timeout% | Avg rounds | DPS | HPS |",
		"| " + classDisplay("warrior") + " | 50 | boss | 0.0% | 50.0% | 50.0% |",
		"Timeout% = fights that hit the 200-round cap",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestMaxSimRoundsCap(t *testing.T) {
	if maxSimRounds < 50 {
		t.Fatalf("maxSimRounds = %d; the cap is what splits stall-outs from real outcomes", maxSimRounds)
	}
}
