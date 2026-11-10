package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Summon: Leviathan's first chapter returns each creature that is not a
// Kraken, Leviathan, Merfolk, Octopus or Serpent to its owner's hand: five
// negated subtypes in one clause. The recorded game casts the Saga beside a
// Faerie, so the chapter's filter keeps the Leviathan and returns the Faerie.
// TestStdSetReprosLockstep replays it.
func filterNegSubWideCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "filter_negsub_five_subtypes_bounces_the_rest",
			prefer: []string{"Summon: Leviathan", "Il Mheg Pixie"},
			decks: [2][]string{
				append(append(deckOf(6, "Summon: Leviathan"), deckOf(6, "Il Mheg Pixie")...), deckOf(12, "Island")...),
				deckOf(24, "Island"),
			},
			surface: "autopay",
			reached: pixieReturnedByChapter,
		},
	}
}

// pixieReturnedByChapter: an Il Mheg Pixie moved from the battlefield to its
// owner's hand, the chapter's return.
func pixieReturnedByChapter(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind != events.MoveZone || e.From != state.ZBattlefield || e.To != state.ZHand {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Il Mheg Pixie" {
			return true
		}
	}
	return false
}

// TestFilterNegSubWideRecord plays the case's policy over seeds until one
// reaches the chapter, and writes the repro. It runs only with
// FILTER_NEGSUB_WIDE_RECORD=1.
func TestFilterNegSubWideRecord(t *testing.T) {
	if os.Getenv("FILTER_NEGSUB_WIDE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fin")
	for _, c := range filterNegSubWideCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-fin"}
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
			t.Fatalf("no seed reached the chapter of %s", c.name)
		})
	}
}

// TestFilterNegSubWideReprosReach replays the recorded game: no divergence,
// every scripted action offered, and the chapter's return reached.
func TestFilterNegSubWideReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range filterNegSubWideCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, cardsettest.Synth(t, "std-fin"), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never returns the Faerie")
			}
		})
	}
}
