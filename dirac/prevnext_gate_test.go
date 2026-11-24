package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrevNextDelayedCounterCreatureGatePlaces: Scars of the Veteran's gated
// delayed rider puts the accumulated counters on the shielded creature while
// it is still a creature, even when an earlier permanent is on the
// battlefield (the target must be the shield's recipient, not object 0 or the
// first permanent).
func TestPrevNextDelayedCounterCreatureGatePlaces(t *testing.T) {
	f := newPrevXScen(t)
	sink := f.perm(f.mite, 1) // an earlier permanent: not the shield's recipient
	bear := f.perm(f.bear, 1)
	if !f.s.isCreature(f.k, f.slot(bear)) || !f.s.isCreature(f.k, f.slot(sink)) {
		t.Fatal("precondition: the target and the decoy are creatures")
	}
	if f.slot(sink) >= f.slot(bear) {
		t.Fatal("precondition: the decoy was not created first")
	}
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRiderDelayedCounter)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	in.W = kop.RefSelf // the lowering's creature-gate marker
	at := installPrevRider(f, &in, bear)
	if at < 0 {
		t.Fatal("precondition: the shield was not installed")
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok || n != 0 {
		t.Fatalf("shield prevented %d (applied %v), want all 2", n, ok)
	}
	if !f.s.isCreature(f.k, f.slot(bear)) {
		t.Fatal("precondition: the target is no longer a creature at the end step")
	}
	f.s.prevEndStepCounters(f.k)
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP0P1)); n != 2 {
		t.Fatalf("gated rider put %d P0P1 counters on the shielded creature, want 2", n)
	}
	if n := f.s.perms[f.slot(sink)].ctrN(f.s, int(kop.CtrP0P1)); n != 0 {
		t.Fatalf("gated rider put %d P0P1 counters on the decoy, want 0", n)
	}
}
