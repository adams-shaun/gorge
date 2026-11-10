package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Mana Flare's "whenever a player taps a land for mana, that player adds one
// mana of any type that land produced" over a Rugged Highlands (R or G), played
// decision by decision against gorge (CR 106.12a, 605.1b): gorge's planner folds
// the extra into each single-colour production of the choice land, the kernel
// prices it as one more mana of the pick (srcReflectPick). TestStdSetReprosLockstep
// replays the repro; TestManaFlareReprosReach requires the recorded game to tap
// a Rugged Highlands for two mana.
//
// The game is recorded by TestManaFlareRecord (MANAFLARE_RECORD=1) with
// effectLongPolicy: the seat with Mana Flare casts it first and otherwise
// picks at random from the seed's stream.
func manaFlareCases() []effectLongCase {
	return []effectLongCase{
		{
			name:    "mana_flare_dual_land_taps_for_one_more_of_its_pick",
			prefer:  []string{"Mana Flare"},
			decks:   [2][]string{deckOf(8, "Mana Flare", deckOf(16, "Rugged Highlands")...), deckOf(8, "Grizzly Bears", deckOf(16, "Forest")...)},
			surface: "autopay",
			reached: dualTapMadeTwo,
		},
	}
}

// dualTapMadeTwo: a Rugged Highlands' tap was followed by two mana additions
// (its own and the reflected extra; nothing else in the decks adds mana).
func dualTapMadeTwo(g *GorgeSide) bool {
	evs := g.E.L.Events
	for i, e := range evs {
		if e.Kind != events.Tap {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Face().Name != "Rugged Highlands" {
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

// TestManaFlareReprosReach replays the recorded game: no divergence, every
// scripted action offered, a Rugged Highlands tapped for two mana.
func TestManaFlareReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range manaFlareCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
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
				t.Fatal("the recorded game never taps a Rugged Highlands for two mana")
			}
		})
	}
}

// TestManaFlareRecord plays the case's policy over seeds until one reaches the
// extra, and writes the repro. It runs only with MANAFLARE_RECORD=1; the game
// it writes is the committed repro.
func TestManaFlareRecord(t *testing.T) {
	if os.Getenv("MANAFLARE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range manaFlareCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
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
