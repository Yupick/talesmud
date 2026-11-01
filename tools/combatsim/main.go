package main

// Command combatsim runs large-scale combat simulations against the real
// TalesMUD combat engine.
//
// Usage (from the repository root):
//
//	go run ./tools/combatsim -iters 1000 -levels 1,10,25,50 -tiers normal,boss
//
// It loads the shipped class catalog (data/classes/*.yaml) and the real skill
// seed, builds class-appropriate characters at each level, then fights them
// against at-level enemies. Output is a markdown table (also written to -out).

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities/skills"
)

func main() {
	iters := flag.Int("iters", 1000, "combats per class/level/tier")
	levelsFlag := flag.String("levels", "1,10,25,50", "comma-separated player levels")
	tiersFlag := flag.String("tiers", "normal,boss", "comma-separated tiers: normal,trash,elite,boss")
	classesFlag := flag.String("classes", "", "comma-separated class ids (default: all six shipped classes)")
	classesDir := flag.String("classes-dir", "data/classes", "directory with class YAML files")
	gearFlag := flag.String("gear", "appropriate", "equipment tier: starter, appropriate, good")
	outPath := flag.String("out", "", "optional markdown output file")
	flag.Parse()

	// The engine is chatty. Keep the tool output clean.
	log.SetLevel(log.ErrorLevel)

	dir, err := resolveClassesDir(*classesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "combatsim: %v\n", err)
		os.Exit(1)
	}
	if n, err := classkit.LoadDir(dir); err != nil {
		fmt.Fprintf(os.Stderr, "combatsim: load classes: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("Loaded %d class definitions from %s\n", n, dir)
	}
	// Populate the skill registry from the real seed (generic + class kit).
	skills.LoadFromDB(skills.SeedSkills())

	classes := parseList(*classesFlag)
	if len(classes) == 0 {
		classes = canonicalClasses
	}
	levels := parseInts(*levelsFlag)
	if len(levels) == 0 {
		levels = []int32{1, 10, 25, 50}
	}
	tiers := parseList(*tiersFlag)
	if len(tiers) == 0 {
		tiers = []string{"normal", "boss"}
	}

	roster, err := buildRoster(levels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "combatsim: %v\n", err)
		os.Exit(1)
	}
	// Filter to the requested classes (buildRoster already used the canonical set).
	want := map[string]bool{}
	for _, c := range classes {
		want[strings.ToLower(c)] = true
	}

	fmt.Printf("Running %d combats per cell: classes=%s levels=%v tiers=%v gear=%s\n\n",
		*iters, strings.Join(classes, ","), levels, tiers, *gearFlag)

	start := time.Now()
	var results []aggregate
	for _, tier := range tiers {
		diff := difficultyFor(tier)
		for _, entry := range roster {
			if !want[entry.classID] {
				continue
			}
			agg := runMatchup(entry, tier, diff, *gearFlag, *iters)
			results = append(results, agg)
			fmt.Printf("  %-9s L%-2d %-7s win=%5.1f%% loss=%5.1f%% to=%5.1f%% rounds=%5.1f dps=%5.1f hps=%5.1f\n",
				entry.name, entry.level, tier, agg.winRate()*100, agg.lossRate()*100, agg.timeoutRate()*100,
				agg.avgRounds(), agg.dps(), agg.hps())
		}
	}
	elapsed := time.Since(start)

	md := renderMarkdown(results, *iters, elapsed)
	fmt.Println()
	fmt.Println(md)

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(md), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "combatsim: write %s: %v\n", *outPath, err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", *outPath)
	}
}

func difficultyFor(tier string) string {
	switch strings.ToLower(tier) {
	case "trash":
		return "easy"
	case "elite":
		return "hard"
	case "boss":
		return "boss"
	default:
		return "normal"
	}
}

func parseList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInts(s string) []int32 {
	var out []int32
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			out = append(out, int32(n))
		}
	}
	return out
}

// resolveClassesDir finds the class directory, walking up from the cwd so the
// tool also works when run from tools/combatsim.
func resolveClassesDir(dir string) (string, error) {
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := cwd; ; {
		candidate := filepath.Join(d, dir)
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("class directory %q not found", dir)
}

func renderMarkdown(results []aggregate, iters int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("### Combat sim results\n\n")
	fmt.Fprintf(&b, "%d combats per cell, %d cells, engine runtime %s.\n\n",
		iters, len(results), elapsed.Round(time.Millisecond))
	b.WriteString("| Class | Level | Tier | Win% | Loss% | Timeout% | Avg rounds | DPS | HPS |\n")
	b.WriteString("|---|---:|---|---:|---:|---:|---:|---:|---:|\n")

	// stable order: class then level then tier
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].class != results[j].class {
			return results[i].class < results[j].class
		}
		if results[i].level != results[j].level {
			return results[i].level < results[j].level
		}
		return results[i].tier < results[j].tier
	})

	for _, r := range results {
		fmt.Fprintf(&b, "| %s | %d | %s | %.1f%% | %.1f%% | %.1f%% | %.1f | %.1f | %.1f |\n",
			classDisplay(r.class), r.level, r.tier,
			r.winRate()*100, r.lossRate()*100, r.timeoutRate()*100,
			r.avgRounds(), r.dps(), r.hps())
	}
	b.WriteString("\nDPS = effective enemy HP removed per combat round. ")
	b.WriteString("HPS = net player HP lost per combat round (heals reduce it). ")
	b.WriteString("Timeout% = fights that hit the 200-round cap with no victor.\n")
	return b.String()
}

func classDisplay(id string) string {
	if d := classkit.Lookup(id); d != nil && d.Name != "" {
		return d.Name
	}
	return id
}
