//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatMBCCombat2Lowers plays four-seat pods of the combat2 fixture
// pool (mbc_combattwo_test.go's decks, alternating around the table): past
// two seats Mage Slayer's damage goes to the seat its equipped attacker
// attacks (the queued defSeat behind AtkPWGuard), a become-blocked pump
// counts that attacker's own blockers, and RemoveFromCombat takes one
// attacker out of a multi-defender combat. No divergence and no kernel stop.
func TestFourSeatMBCCombat2Lowers(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("combat2")
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for s, d := range kcombat2Decks {
		for _, e := range d {
			names[s] = deckOf(e.Count, e.Name, names[s]...)
		}
	}
	decks := [4][]string{names[0], names[1], names[0], names[1]}
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
	play := func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		probe := &combat2Probe{m: m, base: castFirstPolicy(seed)} // no free Reconnaissance pile
		return Lockstep(m, seed, k, PolicyFunc(probe.pick), rc)
	}
	fourSeatAtkClean(t, play, 16)
}
