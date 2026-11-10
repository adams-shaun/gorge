package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Skycloud Expanse's "{1}, {T}: Add {W}{U}" played decision by decision
// against gorge: an all-at-once filter, its card's only mana ability (kernel
// mana_filtereach.go). Seat 0's lands are Plains and Skycloud Expanses, so
// Galina's Knight's {W}{U} is payable only through a Skycloud: a Plains funds
// its {1}, the Skycloud adds {W}{U} at once, no colour ask. gorge's planner
// censuses the Skycloud as a paid gap, so the cast is a scripted prefix the
// autopay surface drops; the game is recorded on the full surface.
//
// The table is built from the repro's own decks (pool "custom"): the frozen
// std-trc pool predates the lowering. TestFilterEachReprosReach requires a
// Knight to resolve onto the battlefield, since a lockstep entry alone would
// pass a script that never activated a Skycloud. The game is recorded by
// TestFilterEachRecord (FILTER_EACH_RECORD=1) and replayed by
// TestStdSetReprosLockstep.
func filterEachCases() []effectLongCase {
	a := append(deckOf(8, "Galina's Knight"), append(deckOf(8, "Skycloud Expanse"), deckOf(8, "Plains")...)...)
	return []effectLongCase{
		{
			name:    "filter_each_skycloud_pays_white_and_blue",
			prefer:  []string{"Galina's Knight"},
			decks:   [2][]string{a, deckOf(24, "Plains")},
			surface: "full",
			reached: knightResolved,
		},
	}
}

// knightResolved: a Galina's Knight entered the battlefield. Its {U} has no
// source on seat 0's side but a Skycloud Expanse's {W}{U}.
func knightResolved(g *GorgeSide) bool {
	evs := g.E.L.Events
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Galina's Knight" {
			return true
		}
	}
	return false
}

// TestFilterEachReprosReach replays the recorded game: no divergence, every
// scripted action offered, a Knight cast off a Skycloud's {W}{U}.
func TestFilterEachReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range filterEachCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never casts a Galina's Knight")
			}
		})
	}
}

// TestFilterEachRecord plays the case's policy over seeds until one reaches
// the cast, and writes the repro. It runs only with FILTER_EACH_RECORD=1; the
// game it writes is the committed repro.
func TestFilterEachRecord(t *testing.T) {
	if os.Getenv("FILTER_EACH_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range filterEachCases() {
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
