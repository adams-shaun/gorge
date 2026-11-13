//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatChosenOpponentLockstep plays four-seat pods of The Rack and
// Black Vise against hands that never empty: as either enters its controller
// chooses one of three opponents (gorge effChoosePlayer's "player" options in
// AliveFrom order; the kernel's ChGenPlayer ask), and only that player's
// upkeep triggers it. No divergence, no stall, the triggers hit, and some
// game chooses an opponent other than the first one listed.
func TestFourSeatChosenOpponentLockstep(t *testing.T) {
	reg := synthReg(t)
	watch := []string{"The Rack", "Black Vise"}
	set := chosenOppPool(t)
	vise := deckOf(10, "Black Vise", deckOf(10, "The Rack", deckOf(20, "Island")...)...)
	lands := deckOf(40, "Plains")
	decks := [4][]string{vise, lands, vise, lands}
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
	code, _ := chooseCode("player")
	pick := ActChoose + CanonKind(code)
	var y chosenOppTally
	asks, farther := 0, 0
	for seed := uint64(1); seed <= 6; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		rc.Finish = func(g *GorgeSide) { y.read(g, watch...) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for _, a := range res.Actions {
			if a.Kind() != pick {
				continue
			}
			asks++
			// seats 0 and 2 cast and each lists the other second (its options
			// are AliveFrom order: 1 2 3 from seat 0, 3 0 1 from seat 2)
			if p := a.Tgt() - RefPlayer; p != 1 && p != 3 {
				farther++
			}
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if asks == 0 || farther == 0 || y.trig == 0 || y.hits == 0 {
		t.Fatalf("opponent asks %d (past the next seat %d), %+v", asks, farther, y)
	}
	t.Logf("opponent asks %d (past the next seat %d), %+v", asks, farther, y)
}
