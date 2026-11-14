package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// DecisionHash keys a DkTarget option's object position in a graveyard or
// exile: gorge's option encoder orders DkTarget objects by their zone index
// there (dirac/feat listpos), so two alike cards at swapped positions are
// different options for the prior cache. ViewHash must not move: ObjHash
// does not tie an object id to its position (marks.go).
func TestDecisionHashKeysPublicZoneOrder(t *testing.T) {
	for _, z := range []kop.Zone{kop.ZoneGraveyard, kop.ZoneExile} {
		sc := newScen(t, Config{})
		a := sc.obj(v.Plains, 0)
		b := sc.obj(v.Plains, 0)
		sc.s.place(a, z, false)
		sc.s.place(b, z, false)
		if ia, ib := sc.s.zoneIndex(a), sc.s.zoneIndex(b); ia != 0 || ib != 1 {
			t.Fatalf("zone %d: zoneIndex %d/%d, want 0/1", z, ia, ib)
		}
		var d Decision
		d.Kind, d.Player, d.N = DkTarget, 0, 2
		d.Options[0] = MakeAction(AkTarget, 0, TCode(a), 0)
		d.Options[1] = MakeAction(AkTarget, 0, TCode(b), 0)
		vh, dh := sc.s.ViewHash(0, nil), sc.s.DecisionHash(&d)
		pl := &sc.s.pl[0]
		switch z {
		case kop.ZoneGraveyard:
			pl.grave[0], pl.grave[1] = pl.grave[1], pl.grave[0]
		case kop.ZoneExile:
			pl.exile[0], pl.exile[1] = pl.exile[1], pl.exile[0]
		}
		if vh2 := sc.s.ViewHash(0, nil); vh2 != vh {
			t.Fatalf("zone %d: swapping two alike cards' positions moved ViewHash (%d vs %d)", z, vh, vh2)
		}
		if dh2 := sc.s.DecisionHash(&d); dh2 == dh {
			t.Fatalf("zone %d: swapping two alike cards' positions left DecisionHash alone", z)
		}
	}
}
