package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGraveTypes: ExGraveTypes counts the distinct card types among the
// controller's graveyard cards (gorge graveyardCardTypeCount).
func TestGraveTypes(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	if got := sc.s.graveTypes(sc.k, 0, 0); got != 0 {
		t.Fatalf("empty graveyard: %d", got)
	}
	for _, c := range []CardID{id["Valley Floodcaller"], id["Valley Floodcaller"], id["Plains"]} {
		o := sc.hand(c, 0)
		sc.s.detach(o)
		sc.s.place(o, kop.ZoneGraveyard, false)
	}
	if got := sc.s.graveTypes(sc.k, 0, 0); got != 2 {
		t.Fatalf("creature, creature, land: %d types, want 2", got)
	}
	if got := sc.s.graveTypes(sc.k, 1, 0); got != 0 {
		t.Fatalf("the opponent's graveyard: %d", got)
	}
}
