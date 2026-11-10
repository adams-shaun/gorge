package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// Roxanne, Starfall Savant's "whenever you tap an artifact token for mana, add
// one mana of any type that artifact token produced" played decision by
// decision against gorge (CR 106.12a, 605.1b): her Meteorite token taps for one
// mana of any colour, and gorge's planner folds the extra into each colour's
// alternative (pay.PaymentPlanReflectedTapMana over the production the
// alternative names), so the tap is two mana of one colour of the payer's
// choice. The kernel prices it on the pick, as one more mana of the colour the
// tap makes (mana_tapextra.go). TestStdSetReprosLockstep replays the repro;
// TestRoxanneReprosReach requires the recorded game to tap an artifact token
// for two mana, since a lockstep entry alone would pass a script that never
// did.
//
// The game is recorded by TestRoxanneRecord (ROXANNE_RECORD=1) on the std-otj
// pool with effectLongPolicy: both seats cast Roxanne first and otherwise pick
// at random from the seed's stream.
func roxanneCases() []effectLongCase {
	return []effectLongCase{
		{
			name:    "roxanne_meteorite_taps_for_two_of_one_colour",
			prefer:  []string{"Roxanne, Starfall Savant"},
			decks:   [2][]string{roxanneDeck(), roxanneDeck()},
			surface: "autopay",
			reached: tokenTapMadeTwo,
		},
	}
}

func roxanneDeck() []string {
	return deckOf(10, "Roxanne, Starfall Savant", append(deckOf(7, "Mountain"), deckOf(7, "Forest")...)...)
}

// tokenTapMadeTwo: an artifact token's tap was followed by two mana additions
// (its own and the reflected extra; nothing else in the decks adds mana
// beside a tap).
func tokenTapMadeTwo(g *GorgeSide) bool {
	evs := g.E.L.Events
	for i, e := range evs {
		if e.Kind != events.Tap {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || !o.IsToken || !slices.Contains(o.Face().Types, "Artifact") {
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

// TestRoxanneReprosReach replays the recorded game: no divergence, every
// scripted action offered, a basic land tapped for two mana.
func TestRoxanneReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-otj")
	for _, c := range roxanneCases() {
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
				t.Fatal("the recorded game never taps an artifact token for two mana")
			}
		})
	}
}

// TestRoxanneRecord plays the case's policy over seeds until one reaches
// the extra, and writes the repro. It runs only with ROXANNE_RECORD=1; the
// game it writes is the committed repro.
func TestRoxanneRecord(t *testing.T) {
	if os.Getenv("ROXANNE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-otj")
	for _, c := range roxanneCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-otj"}
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
