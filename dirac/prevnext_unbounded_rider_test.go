package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrevNextUnboundedCounterRider: the counter rider rides the unbounded
// shield (kop.PrevNextAll|kop.PrevNextRiderCounter, Brace for Impact): each
// take prevents the whole event, hands its full amount to the rider as +1/+1
// counters on the recipient, and leaves the record live.
func TestPrevNextUnboundedCounterRider(t *testing.T) {
	f := newPrevXScen(t)
	sink := f.perm(f.mite, 1) // a decoy first: the rider must not fall back to it
	bear := f.perm(f.bear, 1)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextAll | kop.PrevNextRiderCounter)
	in.Obj = kop.RefTarget0
	at := installPrevRider(f, &in, bear)
	if at < 0 {
		t.Fatal("precondition: the unbounded shield was not installed")
	}
	if col := f.s.g.prevCol[at].col; col != prevNextAll {
		t.Fatalf("precondition: record col = %d, want the unbounded sentinel %d", col, prevNextAll)
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 3); !ok || n != 0 {
		t.Fatalf("first take left %d damage (applied %v), want 0", n, ok)
	}
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP1P1)); n != 3 {
		t.Fatalf("+1/+1 counters after a take of 3 = %d, want 3", n)
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok || n != 0 {
		t.Fatalf("second take left %d damage (applied %v), want 0", n, ok)
	}
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP1P1)); n != 5 {
		t.Fatalf("+1/+1 counters after a second take of 2 = %d, want 5", n)
	}
	if col := f.s.g.prevCol[at].col; col != prevNextAll {
		t.Fatalf("record col after two takes = %d, want still the unbounded sentinel %d", col, prevNextAll)
	}
	if n := f.s.perms[f.slot(sink)].ctrN(f.s, int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("decoy got %d +1/+1 counters, want 0", n)
	}
}
