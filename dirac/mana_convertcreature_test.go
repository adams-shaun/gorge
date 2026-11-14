package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestVizierConversionPaysExactColourFirst pins the floating pool's spend under
// Vizier of the Menagerie's AnyType->AnyType mana conversion (CR 609.4b): gorge's
// ResolveManaWith spends a pip's own colour first, then the generic in C W U B R G
// order, while the may-play any-type rider (payAnyTypePips) spends the first of
// W U B R G C for the pips. Pool {W}{W}{G} paying {1}{G}: the conversion spends
// {G} for the pip and {W} for the generic and leaves {W}; the rider order spends
// {W} for the pip and {W} for the generic and leaves {G}.
func TestVizierConversionPaysExactColourFirst(t *testing.T) {
	const vizier, thespian = "Vizier of the Menagerie", "Wary Thespian"
	sc, id := synthScen(t, "std-fdn", vizier, thespian, "Plains")
	for _, n := range []string{vizier, thespian, "Plains"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s not in the std-fdn pool (run make dirac-tables)", n)
		}
	}
	s := sc.s
	vz := sc.perm(id[vizier], 0)
	o := sc.hand(id[thespian], 0)
	if !sc.onBF(vz) {
		t.Fatal("precondition: Vizier of the Menagerie is not on the battlefield")
	}
	pl := &s.pl[0]
	pl.pool = [kop.NumManaKinds]uint8{kop.W: 2, kop.G: 1}
	d := s.Step()
	var cast Action
	found := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(uint8(o)) {
			cast, found = d.Options[i], true
		}
	}
	if !found {
		t.Fatalf("precondition: Wary Thespian not offered: %v", d.Options[:d.N])
	}
	s.Apply(cast)
	s.Step() // the cast build pays
	if s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("Wary Thespian is in zone %d, not on the stack", s.objs[o].zone)
	}
	if pl.pool[kop.W] != 1 || pl.pool[kop.G] != 0 {
		t.Fatalf("pool after {1}{G}: W=%d G=%d, want W=1 G=0 (exact {G} pip first)", pl.pool[kop.W], pl.pool[kop.G])
	}
}
