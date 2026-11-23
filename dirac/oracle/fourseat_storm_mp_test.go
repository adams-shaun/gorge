package oracle

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestFourSeatStormCountLockstep: Storm (CR 702.40a, "copy it for each other
// spell cast before it this turn") counts every player's spells, not seats 0
// and 1 (gorge's CastThisTurn counts every PutOnStack since the turn began).
// Four-seat pods of mono-blue decks of Flusterstorm, Brainstorm, Arcane Denial
// and Repulse put the trigger over spells cast by seats 2 and 3 (the sweep's
// pod 31: Repulse by seat 3 made the kernel copy nothing). The pods play dirac
// against gorge without divergence, and some game makes two copies in one turn.
func TestFourSeatStormCountLockstep(t *testing.T) {
	deep, d := fourSeatStormPods(t)
	if d != "" {
		t.Fatal(d)
	}
	if deep == 0 {
		t.Fatal("no game reached a storm count of two or more")
	}
	t.Logf("%d of 24 games reached a storm count of two or more", deep)
}

// TestFourSeatStormCountMutation: a storm count read from seats 0 and 1 only
// (dirac.DebugStormTwoSeats) diverges in the same pods.
func TestFourSeatStormCountMutation(t *testing.T) {
	dirac.DebugStormTwoSeats = true
	defer func() { dirac.DebugStormTwoSeats = false }()
	if _, d := fourSeatStormPods(t); d == "" {
		t.Fatal("a two-seat storm count did not diverge")
	}
}

// fourSeatStormPods plays the pods and returns the games that reached a storm
// count of two or more, and the first divergence or stall ("" when none).
func fourSeatStormPods(t *testing.T) (int, string) {
	reg := synthReg(t)
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	// every seat holds Flusterstorm and cheap instants, so the spells cast
	// before a storm trigger span all four seats
	cheap := []string{"Brainstorm", "Arcane Denial", "Repulse"}
	mk := func() []string {
		d := deckOf(10, "Flusterstorm")
		for _, n := range cheap {
			d = append(d, deckOf(8, n)...)
		}
		return append(d, deckOf(13, "Island")...)
	}
	decks := [4][]string{mk(), mk(), mk(), mk()}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	deep := 0 // games with a storm trigger that copied twice or more
	for seed := uint64(1); seed <= 24; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 4000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		run, best := 0, 0
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				evs := c.G.E.L.Events
				run = 0 // copies made since the turn began
				for i := len(evs) - 1; i >= 0 && evs[i].Kind != events.TurnChange; i-- {
					if evs[i].Kind == events.StackCopy {
						run++
					}
				}
				best = max(best, run)
			}
			return RandomPolicy(seed).Pick(c)
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			return deep, fmt.Sprintf("seed %d: divergence at %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
		if res.StallUnmodelled {
			return deep, fmt.Sprintf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if best >= 2 {
			deep++
		}
	}
	return deep, ""
}
