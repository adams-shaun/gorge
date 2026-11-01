package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterDestination: OpCounter's C (Destination$) moves the countered
// spell card to exile, its owner's hand, or its library top or bottom
// (gorge effCounter); a flashback spell is exiled whatever the destination.
func TestCounterDestination(t *testing.T) {
	for _, c := range []struct {
		dest int16
		fb   bool
		zone kop.Zone
	}{{kop.CtrDestExile, false, kop.ZoneExile}, {kop.CtrDestHand, false, kop.ZoneHand},
		{kop.CtrDestLibTop, false, kop.ZoneLibrary}, {kop.CtrDestLibBottom, false, kop.ZoneLibrary},
		{kop.CtrDestHand, true, kop.ZoneExile}} {
		sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
		fc := id["Valley Floodcaller"]
		o := sc.hand(fc, 1)
		sc.s.detach(o)
		sc.s.objs[o].zone = kop.ZoneStack
		e := stackEntry{kind: seSpell, obj: o, card: fc, ctrl: 1, trigObj: NoObj}
		if c.fb {
			e.flags |= sfFlashback
		}
		sc.s.stack[0], sc.s.sp = e, 1
		sc.s.counterTo(sc.k, 0, c.dest)
		pl := &sc.s.pl[1]
		if sc.s.sp != 0 || sc.s.objs[o].zone != c.zone {
			t.Fatalf("dest %d flashback %v: sp %d zone %d, want %d", c.dest, c.fb, sc.s.sp, sc.s.objs[o].zone, c.zone)
		}
		if c.zone == kop.ZoneLibrary {
			top, bottom := pl.lib[pl.libLo], pl.lib[pl.libHi-1]
			if c.dest == kop.CtrDestLibTop && top != o || c.dest == kop.CtrDestLibBottom && bottom != o {
				t.Fatalf("dest %d: not placed at the right end of the library", c.dest)
			}
		}
	}
}
