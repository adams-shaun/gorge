package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEachPlayerSacrificeLookBack: Liliana, Dreadhorde General's -4 ("each
// player sacrifices two creatures") is one departure window in gorge (the
// effect's batch window parks the pre-departure board, CR 603.10a): p0's
// forced batch (High-Society Hunter and a Rat) goes to the graveyard
// before p1 is asked, yet the Hunter still sees p1's two Rats die. The
// Hunter triggers three times (p0's Rat, then p1's two). Before, the
// departed Hunter missed p1's batch (pairs 66377 67298 84090, NStack
// 59727 80962).
func TestEachPlayerSacrificeLookBack(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Liliana, Dreadhorde General", Count: 1}, {Name: "High-Society Hunter", Count: 1}, {Name: "Burglar Rat", Count: 4}, {Name: "Swamp", Count: 8}})
	li := sc.perm(id["Liliana, Dreadhorde General"], 0)
	sc.s.perms[sc.slot(li)].setCtrN(sc.s, int(kop.CtrLoyalty), 6)
	hunter := sc.perm(id["High-Society Hunter"], 0)
	sc.perm(id["Burglar Rat"], 0)
	for i := 0; i < 3; i++ {
		sc.perm(id["Burglar Rat"], 1)
	}
	sc.s.npend = 0
	if sc.s.active != 0 {
		t.Skip("p0 is not active")
	}
	d := sc.s.Step()
	act := false
	for j := 0; j < int(d.N); j++ {
		if a := d.Options[j]; a.Kind() == AkActivate && a.Source() == TCode(uint8(li)) && a.Mode() == 1 {
			sc.s.Apply(a) // the -4
			act = true
			break
		}
	}
	if !act {
		t.Fatalf("Liliana's -4 is not offered: %v", d.Options[:d.N])
	}
	asked := false
	for i := 0; i < 20 && !asked; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Player == 1 {
			asked = true
			sc.s.Apply(d.Options[0])
			d = sc.s.Step()
			sc.s.Apply(d.Options[0])
			break
		}
		pick := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			if a := d.Options[j]; a.Kind() == AkPass {
				pick = a
			}
		}
		sc.s.Apply(pick)
	}
	if !asked {
		t.Fatal("p1 was never asked to sacrifice")
	}
	n := 0
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].obj == hunter {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("High-Society Hunter triggered %d times, want 3 (p0's Rat, then p1's two)", n)
	}
}
