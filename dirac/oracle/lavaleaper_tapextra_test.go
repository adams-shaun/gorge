package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// Lavaleaper's "whenever a player taps a basic land for mana, that player
// adds one mana of any type that land produced" played decision by decision
// against gorge (CR 106.12a, 605.1b): gorge's planner folds the extra into a
// basic land's tap (pay.PaymentPlanReflectedTapMana), the kernel adds it
// through a StTapReflect | StTapAnyPlayer StatTapExtra. TestStdSetReprosLockstep
// replays the repro; TestLavaleaperReprosReach requires the recorded game to
// tap a basic land for two mana, since a lockstep entry alone would pass a
// script that never did.
//
// The game is recorded by TestLavaleaperRecord (LAVALEAPER_RECORD=1) on the
// std-ecl pool with effectLongPolicy: both seats cast Lavaleaper first and
// otherwise pick at random from the seed's stream.
func lavaleaperCases() []effectLongCase {
	return []effectLongCase{
		{
			name:    "lavaleaper_basic_land_taps_for_one_more",
			prefer:  []string{"Lavaleaper"},
			decks:   [2][]string{deckOf(10, "Lavaleaper", deckOf(14, "Mountain")...), deckOf(10, "Lavaleaper", deckOf(14, "Mountain")...)},
			surface: "autopay",
			reached: basicTapMadeTwo,
		},
	}
}

// basicTapMadeTwo: a basic land's tap was followed by two mana additions
// (the land's own and the reflected extra; nothing else in the decks adds
// mana beside a tap).
func basicTapMadeTwo(g *GorgeSide) bool {
	evs := g.E.L.Events
	for i, e := range evs {
		if e.Kind != events.Tap {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || !slices.Contains(o.Face().Types, "Land") {
			continue
		}
		n := 0
		for j := i + 1; j < len(evs) && j <= i+6; j++ {
			if evs[j].Kind == events.ManaAdd {
				n++
			}
			if evs[j].Kind == events.Tap {
				break
			}
		}
		if n >= 2 {
			return true
		}
	}
	return false
}

// TestLavaleaperReprosReach replays the recorded game: no divergence, every
// scripted action offered, a basic land tapped for two mana.
func TestLavaleaperReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	for _, c := range lavaleaperCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never taps a basic land for two mana")
			}
		})
	}
}

// TestLavaleaperRecord plays the case's policy over seeds until one reaches
// the extra, and writes the repro. It runs only with LAVALEAPER_RECORD=1; the
// game it writes is the committed repro.
func TestLavaleaperRecord(t *testing.T) {
	if os.Getenv("LAVALEAPER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	for _, c := range lavaleaperCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-ecl"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
