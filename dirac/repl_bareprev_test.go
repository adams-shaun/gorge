package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBarePreventionsNoOrderAsk: Seismic Rupture sweeps a board where one
// player has two Crystal Barricades. Each of that player's lions is matched
// by two bare preventions; whichever applies, the damage is gone, so gorge
// poses no CR 616.1 order ask (engine/rules allBarePreventions) and neither
// does the kernel: the spell resolves without a pause and the lions take
// nothing.
func TestBarePreventionsNoOrderAsk(t *testing.T) {
	deck := []cardset.Entry{{Name: "Seismic Rupture", Count: 1}, {Name: "Crystal Barricade", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Mountain", Count: 10}}
	sc, id := replScen(t, deck)
	s := sc.s
	b0 := sc.perm(id["Crystal Barricade"], 1)
	l0 := sc.perm(id["Savannah Lions"], 1)
	l1 := sc.perm(id["Savannah Lions"], 1)
	b1 := sc.perm(id["Crystal Barricade"], 1)
	mine := sc.perm(id["Savannah Lions"], 0)
	if b0 == b1 || !sc.onBF(b0) || !sc.onBF(b1) {
		t.Fatalf("precondition: want two Barricades on the battlefield, got %d and %d", b0, b1)
	}
	sc.lands(0, id["Mountain"], 3)
	sr := sc.hand(id["Seismic Rupture"], 0)
	sc.do(t, AkCast, TCode(sr), scAny)

	for i := 0; i < 12; i++ {
		d := s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			t.Fatalf("order ask posed between two bare preventions: player %d opts %v", d.Player, d.Options[:d.N])
		}
		if d.Kind == DkPriority && s.sp > 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if d.Kind == DkPriority && s.sp == 0 {
			break
		}
		t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
	}
	if s.objs[sr].zone != kop.ZoneGraveyard || s.sp != 0 {
		t.Fatalf("spell zone %d stack %d, want it resolved", s.objs[sr].zone, s.sp)
	}
	for _, l := range []Obj{l0, l1} {
		if !sc.onBF(l) || s.perms[sc.slot(l)].damage != 0 {
			t.Fatalf("lion %d: on battlefield %v damage %d, want it prevented", l, sc.onBF(l), s.perms[sc.slot(l)].damage)
		}
	}
	if sc.onBF(mine) {
		t.Fatal("precondition: the caster's own lion (no Barricade) survived the 2 damage; the sweep did not deal damage")
	}
}
