package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFirstPip: a hybrid symbol offers its colours clockwise from the first the
// symbol prints, whichever the kernel lists first.
func TestFirstPip(t *testing.T) {
	const w, u, b, r, g = 0, 1, 2, 3, 4
	for _, tc := range []struct{ x, y, want uint8 }{
		{w, u, w}, {u, b, u}, {b, r, b}, {r, g, r}, {g, w, g},
		{w, b, w}, {u, r, u}, {b, g, b}, {r, w, r}, {g, u, g},
	} {
		var d dirac.Decision
		d.N = 2
		d.Options[0] = dirac.MakeAction(dirac.AkChoose, 0, 0, tc.x)
		d.Options[1] = dirac.MakeAction(dirac.AkChoose, 0, 0, tc.y)
		if got := d.Options[firstPip(&d)].Mode(); got != tc.want {
			t.Errorf("{%d/%d}: first %d, want %d", tc.x, tc.y, got, tc.want)
		}
	}
}

// TestObjKeyPlayerCode: a player's target code (a proliferate or Aura
// recipient's) names no object, so its key reads none and follows every
// object's, by seat, as gorge lists players after permanents; FeatZoneIndex
// places no non-object code.
func TestObjKeyPlayerCode(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Forest", Count: 20}, {Name: "Llanowar Elves", Count: 20}}
	tbl, da, db, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	s := dirac.NewConfig(tbl, da, db, 1, dirac.Config{})
	bt := New(NewFacts(tbl))
	bt.b.fillPos(s)
	var maxObj uint32
	for o := 0; o < s.NumObjs(); o++ {
		maxObj = max(maxObj, bt.objKey(s, dirac.Obj(o)))
	}
	prev := maxObj
	for p := uint8(0); p < s.NumSeats(); p++ {
		c := dirac.PlayerCode(p)
		if k := bt.objKey(s, dirac.Obj(c)); k <= prev {
			t.Fatalf("seat %d's code keys %#x, not past %#x", p, k, prev)
		} else {
			prev = k
		}
		if i := s.FeatZoneIndex(dirac.Obj(c)); i != 255 {
			t.Fatalf("FeatZoneIndex(seat %d's code) = %d, want 255", p, i)
		}
	}
	if i := s.FeatZoneIndex(dirac.Obj(dirac.StackTarget0)); i != 255 {
		t.Fatalf("FeatZoneIndex(a stack ability's code) = %d, want 255", i)
	}
}
