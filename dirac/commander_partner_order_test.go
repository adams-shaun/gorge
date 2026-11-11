package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// partnerCastOrder is the command-zone casts p's priority walk offers, in
// offer order, as source objects.
func partnerCastOrder(sc *scen, p uint8) []Obj {
	sc.s.prio = p
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	var out []Obj
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast {
			out = append(out, Obj(a.Source()))
		}
	}
	return out
}

// TestCommanderPartnerOfferOrder: gorge walks its command zone as an ordered
// list, so the commander that arrived last is offered second; the canonical
// option key sorts by object id and hides the order from the lockstep.
func TestCommanderPartnerOfferOrder(t *testing.T) {
	sc := partnerScen(t)
	s := sc.s
	a, b := s.FeatCommanderN(0, 0), s.FeatCommanderN(0, 1)
	for i := 0; i < 8; i++ {
		sc.perm(v.Forest, 0)
	}
	same := func(got, want []Obj) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if got := partnerCastOrder(sc, 0); !same(got, []Obj{a, b}) {
		t.Fatalf("at genesis offered %v, want %v (first commander, then the partner)", got, []Obj{a, b})
	}
	castCommanderFromCommandZone(t, sc, a)
	returnCommanderToCommandZone(t, sc, a)
	s.noteCommandZone(a) // the zone move's own bookkeeping (zones.go)
	if got := partnerCastOrder(sc, 0); !same(got, []Obj{b, a}) {
		t.Fatalf("after the first commander re-entered offered %v, want %v", got, []Obj{b, a})
	}
}

// TestCommanderFeatNoPartnerRecord: a game with no partner record (every
// Constructed game) reads zero for the partner slot and slot 0's value for the
// seat totals, never a nil dereference.
func TestCommanderFeatNoPartnerRecord(t *testing.T) {
	d := cmdTestDecks()
	s := NewConfig(v.Table, d[0], d[1], cmdTestSeed, Config{NoElide: true})
	if s.partnerCold() != nil {
		t.Fatal("a Constructed game carries a partner record")
	}
	s.pl[0].cmdCast, s.pl[1].cmdDmg[0] = 3, 9
	if s.FeatCommanderCast(0) != 3 || s.FeatCommanderCastN(0, 0) != 3 || s.FeatCommanderCastN(0, 1) != 0 {
		t.Fatalf("casts %d %d %d, want 3 3 0", s.FeatCommanderCast(0), s.FeatCommanderCastN(0, 0), s.FeatCommanderCastN(0, 1))
	}
	if s.FeatCommanderDamage(1, 0) != 9 || s.FeatCommanderDamageN(1, 0, 0) != 9 || s.FeatCommanderDamageN(1, 0, 1) != 0 {
		t.Fatalf("damage %d %d %d, want 9 9 0", s.FeatCommanderDamage(1, 0), s.FeatCommanderDamageN(1, 0, 0), s.FeatCommanderDamageN(1, 0, 1))
	}
}
