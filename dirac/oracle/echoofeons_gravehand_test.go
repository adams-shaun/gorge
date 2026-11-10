package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Hand,Graveyard sweeps over every player (Echo of Eons, Timetwister,
// Time Spiral, Time Reversal, Day's Undoing: ChangeZoneAll Card, UseAllOriginZones$,
// Shuffle$ True) put every hand and graveyard into its owner's library, the
// hands first, then shuffle the owner of the first card moved and the other
// seat (gorge changeZoneAllShuffleLibraries). The pre-shuffle order and the
// shuffle order both feed the seeded shuffle, so a library that differs by
// one card shows at the following draw.
// TestStdSetReprosLockstep replays one recorded game of each;
// TestHandGraveSweepReprosReach requires the recorded game to shuffle both
// libraries after the card resolves, since a lockstep entry alone would pass a
// script that never cast it.
//
// The games are recorded by TestHandGraveSweepRecord (HAND_GRAVE_SWEEP_RECORD=1)
// on charmRepeatPolicy: both seats cast the case's card first.
type handGraveCase struct{ name, card string }

func handGraveCases() []handGraveCase {
	return []handGraveCase{
		{"hand_grave_sweep_echo_of_eons", "Echo of Eons"},
		{"hand_grave_sweep_timetwister", "Timetwister"},
		{"hand_grave_sweep_time_reversal", "Time Reversal"},
		{"hand_grave_sweep_days_undoing", "Day's Undoing"},
	}
}

// handGraveRefused is Time Spiral, which the sweep lowers but whose own
// "untap up to six lands" pick after exiling itself from the stack leaves the
// engines' stacks differing at the pick (dirac/support/refused/time-spiral.json
// pins this game). The sweep itself is not what differs: the recorded game
// reaches it first.
var handGraveRefused = handGraveCase{"time_spiral_untap_pick_stack_differs", "Time Spiral"}

func (c handGraveCase) decks() [2][]string {
	return [2][]string{
		append(deckOf(6, c.card), deckOf(18, "Island")...),
		append(deckOf(5, c.card), deckOf(19, "Island")...),
	}
}

// handGraveShuffles is the number of Shuffle events between the first
// resolution of the case's card and the next resolution: the sweep shuffles
// both libraries (a shuffle only logged by the sweep, not by the game's
// start).
func handGraveShuffles(g *GorgeSide, card string) int {
	evs := g.E.L.Events
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.Resolve {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != card {
			continue
		}
		n := 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind == events.Shuffle {
				n++
			}
		}
		return n
	}
	return 0
}

func handGravePlay(t *testing.T, reg *cards.Registry, c handGraveCase, r *Repro, set cardset.Set, pol Policy) (Result, int) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000 // recording
	}
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	shuffles := 0
	rc.Finish = func(g *GorgeSide) { shuffles = handGraveShuffles(g, c.card) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), shuffles
}

// TestHandGraveSweepReprosReach replays the recorded games: no divergence,
// every scripted action offered, and both libraries shuffled by the sweep.
func TestHandGraveSweepReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range handGraveCases() {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, n := handGravePlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if n < 2 {
				t.Fatalf("%d library shuffles after %s resolved, want both libraries", n, c.card)
			}
		})
	}
}

// TestHandGraveSweepRecord plays the cases' scripted policies over seeds until
// one reaches the sweep, and writes the repro. It runs only with
// HAND_GRAVE_SWEEP_RECORD=1; the games it writes are the committed repros.
func TestHandGraveSweepRecord(t *testing.T) {
	if os.Getenv("HAND_GRAVE_SWEEP_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range append(handGraveCases(), handGraveRefused) {
		t.Run(c.card, func(t *testing.T) {
			decks := c.decks()
			set := charmRepeatSet(t, decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, n := handGravePlay(t, reg, c, r, set, charmRepeatPolicy(m, charmRepeatCase{name: c.name, card: c.card, decks: decks}))
				if res.Div != nil && c != handGraveRefused {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if c == handGraveRefused {
					if res.Div == nil {
						continue
					}
					t.Logf("seed %d: diverges at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
				} else if n < 2 {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d shuffles", seed, len(res.Actions), n)
				return
			}
			t.Fatalf("no seed reached %s's sweep", c.card)
		})
	}
}
