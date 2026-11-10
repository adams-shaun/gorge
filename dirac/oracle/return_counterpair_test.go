package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Perennation played decision by decision against gorge: a return from the
// graveyard whose WithCountersType$ is a two-kind list, so the permanent
// enters with a hexproof counter and an indestructible counter, placed in
// script order as part of the entry (CR 122.6, 614.1c). Rainveil Rejuvenator's
// enters mill puts permanent cards into the graveyard for the spell to find.
//
// TestReturnCounterPairReprosReach requires the recorded game to put both
// counters on one returned permanent, hexproof first; a lockstep entry alone
// would pass a script that never resolved Perennation. The game is recorded by
// TestReturnCounterPairRecord (RETURN_COUNTER_PAIR_RECORD=1) on the std-tdm
// pool and replayed by TestStdSetReprosLockstep.
func returnCounterPairCases() []effectLongCase {
	a := append(deckOf(4, "Perennation"), append(deckOf(4, "Rainveil Rejuvenator"), append(deckOf(3, "Sinkhole Surveyor"),
		append(deckOf(4, "Plains"), append(deckOf(4, "Swamp"), deckOf(5, "Forest")...)...)...)...)...)
	return []effectLongCase{
		{
			name:    "return_enters_with_hexproof_and_indestructible_counters",
			prefer:  []string{"Perennation", "Rainveil Rejuvenator", "Sinkhole Surveyor"},
			decks:   [2][]string{a, append(deckOf(6, "Krotiq Nestguard"), deckOf(18, "Forest")...)},
			surface: "autopay",
			reached: returnedWithCounterPair,
		},
	}
}

// returnedWithCounterPair: one object took a hexproof counter and then an
// indestructible counter, the order of Perennation's list. Nothing else in
// the decks places either kind.
func returnedWithCounterPair(g *GorgeSide) bool {
	hex, ind := state.CanonicalCounterKind("Hexproof"), state.CanonicalCounterKind("Indestructible")
	evs := g.E.L.Events
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.CounterChange || e.Counter != hex || e.Amount <= 0 {
			continue
		}
		for j := i + 1; j < len(evs); j++ {
			f := &evs[j]
			if f.Kind == events.CounterChange && f.Counter == ind && f.Amount > 0 && f.Obj == e.Obj {
				return true
			}
		}
	}
	return false
}

// TestReturnCounterPairReprosReach replays the recorded game: no divergence,
// every scripted action offered, a permanent returned with both counters.
func TestReturnCounterPairReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tdm")
	for _, c := range returnCounterPairCases() {
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
				t.Fatal("the recorded game never returns a permanent with a hexproof and an indestructible counter")
			}
		})
	}
}

// TestReturnCounterPairRecord plays the case's policy over seeds until one
// reaches the return, and writes the repro. It runs only with
// RETURN_COUNTER_PAIR_RECORD=1; the game it writes is the committed repro.
func TestReturnCounterPairRecord(t *testing.T) {
	if os.Getenv("RETURN_COUNTER_PAIR_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tdm")
	for _, c := range returnCounterPairCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-tdm"}
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
