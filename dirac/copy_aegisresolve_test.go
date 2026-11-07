package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAegisCopyOrderedByResolutionTick: the Aegis was attached at 10, an
// instant clone resolved at 20, and the Aegis's trigger resolved last (30).
// gorge stamps the Aegis copy at resolution, so it is the newest layer.
func TestAegisCopyOrderedByResolutionTick(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.HillGiant, 0)
	sl := sc.slot(bearer)
	addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	s.g.links[0].ts = 30 // resolved after the clone
	addCopyEOT(sc, sl, v.GrizzlyBears, NoObj, 0, 20)
	if l := s.copyTop(sl, NoSlot); l.kind != copyAegis || l.card != v.SerraAngel {
		t.Fatalf("top layer kind %d card %d, want the Aegis copy (resolved last)", l.kind, l.card)
	}
	s.g.links[0].ts = 5 // resolved before the clone
	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT {
		t.Fatalf("top layer kind %d, want the clone (newer)", l.kind)
	}
}
