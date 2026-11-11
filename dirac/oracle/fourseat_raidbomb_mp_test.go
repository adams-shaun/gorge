//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatOtherAttackerDefenderLockstep plays four-seat pods of the
// combat fixture pool around Raid Bombardment: its Attacks trigger is
// another attacker's, and its damage goes to the player that attacker
// attacks. Each trigger is queued with its own attack event, so past two
// seats its defending player is that event's seat: no divergence and no
// stall. Without the bound seat (dirac.DebugDropDefSeat) the damage goes to
// the seat after the controller and some seed diverges.
func TestFourSeatOtherAttackerDefenderLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("mcombat")
	if err != nil {
		t.Fatal(err)
	}
	red := deckOf(8, "Raid Bombardment", deckOf(8, "Goblin Piker", deckOf(6, "Gray Ogre", deckOf(18, "Mountain")...)...)...)
	white := deckOf(12, "Savannah Lions", deckOf(6, "Grizzly Bears", deckOf(10, "Plains", deckOf(12, "Forest")...)...)...)
	decks := [4][]string{red, white, red, white}
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
		return Lockstep(m, seed, k, RandomPolicy(seed), rc)
	}
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugDropDefSeat) {
		t.Fatal("dropping the queued defending seat changed no seed")
	}
}
