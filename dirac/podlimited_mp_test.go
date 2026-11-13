//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCardPodLimitedIsPrepTwoSeat: kop.CardPodLimited, which kgen -supported
// -four-seat reads to keep such cards out of a four-seat deck pool (decks.py
// --four-seat), names exactly the cards whose one-card table the four-seat
// build refuses at a game's start (prepTwoSeat, prepGoad: StallTwoSeat), over every
// entry of the mbc and cmd5 tables.
func TestCardPodLimitedIsPrepTwoSeat(t *testing.T) {
	limited, n := 0, 0
	for _, name := range []string{"mbc", "cmd5"} {
		set, err := cardset.ByName(name)
		if err != nil {
			t.Skipf("no %s table (make dirac-tables): %v", name, err)
		}
		for i := range set.Names() {
			d := *set.Def(i)
			tb := kop.Table{Cards: []kop.CardDef{d}}
			var g genPrep
			prepTwoSeat(&tb, &g)
			prepGoad(&tb, &g)
			if got := kop.CardPodLimited(&d); got != g.twoSeatOnly {
				t.Errorf("%s %s: CardPodLimited %v, prepTwoSeat %v", name, set.Names()[i], got, g.twoSeatOnly)
			}
			n++
			if g.twoSeatOnly {
				limited++
			}
		}
	}
	t.Logf("%d entries, %d limited to two seats", n, limited)
	if limited == 0 {
		t.Fatal("no entry is limited: the comparison is vacuous")
	}
}
