package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrLifePairCast puts the named permanents and a Plains on seat 0's empty
// board in that order and casts Divination ({2}{U}, a sorcery) from hand.
// Only Great Hall of the Biblioplex's restriction admits a sorcery, so the
// cast pays one life through it and nothing through an unadmitted pair.
func restrLifePairCast(t *testing.T, board ...string) (zone kop.Zone, life int) {
	t.Helper()
	names := append([]string{"Divination"}, board...)
	sc, id := restrLifeAnyScen(t, names...)
	for _, n := range board {
		sc.perm(id[n], 0)
	}
	sc.perm(id["Plains"], 0)
	div := sc.hand(id["Divination"], 0)
	if !mayCastHand(sc, div) {
		t.Fatalf("%v: Divination is not offered", board)
	}
	return sc.s.objs[div].zone, int(sc.s.pl[0].life)
}

// mayCastHand drives the offered cast of the hand card o through its
// decisions to its resolution, reporting whether the offer listed it.
func mayCastHand(sc *scen, o Obj) bool {
	d := sc.s.Step()
	found := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		return false
	}
	for i := 0; i < 12; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
		if sc.s.objs[o].zone == kop.ZoneGraveyard {
			break
		}
	}
	return true
}

// A pair carrier whose restriction the payment does not admit stays an
// ordinary {C} unit: it must not join the paired type of an admitted carrier
// (the planner's cap would count it as able to take the painful alternative),
// whatever the battlefield order.
func TestRestrictedLifePairMixedBoardAdmitsPerPayment(t *testing.T) {
	for _, board := range [][]string{
		{"Great Hall of the Biblioplex", "Voldaren Estate"},
		{"Voldaren Estate", "Great Hall of the Biblioplex"},
	} {
		zone, life := restrLifePairCast(t, board...)
		if zone != kop.ZoneGraveyard || life != 19 {
			t.Errorf("%v: Divination zone=%d life=%d, want graveyard (%d) and life 19", board, zone, life, kop.ZoneGraveyard)
		}
	}
}

// An unrestricted painland beside a restricted pair that the payment admits
// and one it does not: the plan pays one life through whichever pair makes the
// blue, and the unadmitted pair stays out of the others' paired type, in every
// battlefield order.
func TestRestrictedLifePairBesideUnrestrictedPainland(t *testing.T) {
	boards := [][]string{
		{"Underground River", "Great Hall of the Biblioplex", "Voldaren Estate"},
		{"Voldaren Estate", "Underground River", "Great Hall of the Biblioplex"},
		{"Great Hall of the Biblioplex", "Voldaren Estate", "Underground River"},
		{"Voldaren Estate", "Great Hall of the Biblioplex", "Underground River"},
	}
	for _, board := range boards {
		zone, life := restrLifePairCast(t, board...)
		if zone != kop.ZoneGraveyard || life != 19 {
			t.Errorf("%v: Divination zone=%d life=%d, want graveyard (%d) and life 19", board, zone, life, kop.ZoneGraveyard)
		}
	}
}
