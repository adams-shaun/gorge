package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttackTriggersPerDefender: gorge emits one DeclareAttackers event per
// defender (the player first, then each planeswalker) and queues each
// event's attack triggers before the next event's (rules/combat.go byDef;
// the pair 7031). Two Sanguine Syphoners, the older attacking the
// opponent's planeswalker and the newer the opponent: the newer's trigger
// is queued first. Before, the kernel queued them in listener order.
func TestAttackTriggersPerDefender(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sanguine Syphoner", Count: 2}, {Name: "Liliana, Dreadhorde General", Count: 1}, {Name: "Swamp", Count: 8}})
	old := sc.perm(id["Sanguine Syphoner"], 0)
	nw := sc.perm(id["Sanguine Syphoner"], 0)
	li := sc.perm(id["Liliana, Dreadhorde General"], 1)
	sc.s.perms[sc.slot(li)].setCtrN(sc.s, int(kop.CtrLoyalty), 6)
	declared := false
	var first Obj = 255
	sc.stepTo(t, 30, func() bool { return first != 255 }, func(d *Decision) (Action, bool) {
		if declared && sc.s.npend >= 2 {
			first = sc.s.pend[0].obj
			return 0, false
		}
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			switch {
			case d.Kind == DkAttackers && a.Kind() == AkAttack && a.Source() == TCode(uint8(old)) && a.Target() == TCode(uint8(li)) && sc.s.perms[sc.slot(old)].flags&pfAttacking == 0:
				return a, true
			case d.Kind == DkAttackers && a.Kind() == AkAttack && a.Source() == TCode(uint8(nw)) && a.Target() >= PlayerTarget0 && sc.s.perms[sc.slot(nw)].flags&pfAttacking == 0:
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; d.Kind == DkAttackers && a.Kind() == AkDone {
				declared = true
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	t.Logf("npend %d first %d (old %d new %d)", sc.s.npend, first, old, nw)
	if first != nw {
		t.Fatalf("first queued attack trigger is from %d, want the newer Syphoner %d (it attacks the player)", first, nw)
	}
}
