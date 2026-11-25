package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Castle-only restricted mana-fee board: five untapped Castle Garenbrigs
// and no Forest must still offer a Craw Wurm. Four Castles tap for their plain
// {T}: Add {G} and the fifth pays its {2}{G}{G} fee from that floating mana
// (CR 605.3b), producing six restricted {G} for the Wurm's {4}{G}{G}. The
// deck is Castle-dense on purpose (24 Castles, 4 Forests): the fee is funded
// by another Castle's census ability, never by a Forest or an Elf, which is
// the case TestRestrictedManaFeeThresholdLockstep does not reach.

// restrictedManaFeeCastleBoardDeck is the Castle-heavy deck (40 cards).
func restrictedManaFeeCastleBoardDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Craw Wurm", Count: 12}, {Name: "Castle Garenbrig", Count: 24}, {Name: "Forest", Count: 4},
	}
}

// newRestrictedManaFeeCastleBoardPod builds a pod of seats copies of the deck.
func newRestrictedManaFeeCastleBoardPod(t *testing.T, seats int) restrictedManaPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("restrictedmana")
	if err != nil {
		t.Fatal(err)
	}
	p := restrictedManaPod{reg: reg, set: set}
	for i := 0; i < seats; i++ {
		p.decks = append(p.decks, restrictedManaFeeCastleBoardDeck())
	}
	return p
}

// TestRestrictedManaFeeCastleBoardLockstep plays the Castle-only fee pod over
// seeds 1-20, two and four seats: no divergence and no stall, and a Castle's
// mana-fee restricted mana was spent over the seeds.
func TestRestrictedManaFeeCastleBoardLockstep(t *testing.T) {
	for _, seats := range []int{2, 4} {
		p := newRestrictedManaFeeCastleBoardPod(t, seats)
		fee := 0
		for seed := uint64(1); seed <= 20; seed++ {
			res, c := p.play(t, seed, 40)
			if res.Div != nil {
				restrictedManaFail(t, seed, res)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("%d seats seed %d: kernel stopped: %s", seats, seed, res.Stall)
			}
			fee += c.byClass[rmFee]
			t.Logf("%d seats seed %d: %d decisions; spent fee %v by %v", seats, seed, res.Decisions, c.byClass, c.byName)
		}
		if fee == 0 {
			t.Fatalf("%d seats: no Castle Garenbrig mana-fee restricted mana was spent over the seeds", seats)
		}
	}
}
