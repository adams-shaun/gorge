package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// resolveBareAll registers a bare unbounded PreventNext shield on rcpt (the
// Indestructible Aura shape: kop.PrevNextAll, no PrevNextFrom and no rider).
func (f *prevXScen) resolveBareAll(rcpt TCode) {
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextAll)
	in.Obj = kop.RefTarget0
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	e.tgt[0] = rcpt
	f.s.prevNextAdd(f.k, &in)
}

// bareShieldOn returns the record index of the single bare unbounded shield
// matching the event, or -1.
func (f *prevXScen) bareShieldOn(code TCode, combat bool) int {
	for i := range f.s.g.prevCol {
		if f.s.pnxBare(i) && f.s.pnxMatches(i, code, f.s.objRef(f.k, kop.RefSelf), combat) {
			return i
		}
	}
	return -1
}

// TestPrevNextBareShieldBesideGainRider: a bare unbounded shield beside a
// matching colour/scope record that carries a rider is not gorge's bare
// prevention pair. allBarePreventions rejects the rider record (its
// PreventionEffect$ True has a ReplaceWith$ body), so gorge asks the CR 616.1
// order and a pick can gain life; the kernel has no order ask there and must
// stall StallPrevNext instead of applying the bare shield whole. A riderless
// scope record beside the same shield is ask-free and must not stall.
func TestPrevNextBareShieldBesideGainRider(t *testing.T) {
	t.Run("gain rider competes", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant() // PrevScopeAll|SrcCreature|Gain: a gain-rider record for seat 0
		f.resolveBareAll(PlayerCode(0))
		bear := f.perm(f.bear, 1)
		f.bearSrc(bear)
		srcSl := f.slot(bear)
		tgt := prevTgt{0, NoSlot}
		if s.g.prevX[0]&pxRider == 0 || !s.prevRecHit(f.k, 0, srcSl, f.bear, tgt, true) {
			t.Fatal("precondition: the gain-rider scope record does not match the event")
		}
		at := f.bareShieldOn(PlayerCode(0), true)
		if at < 0 {
			t.Fatal("precondition: no bare unbounded shield matches the event")
		}
		if n, ok := s.prevNextTake(f.k, 1, srcSl, f.bear, PlayerCode(0), 0, NoSlot, true, 3); ok {
			t.Fatalf("the bare shield applied whole (%d dealt) beside a gain-rider scope record", n)
		}
		if got := s.StallReason(); got != StallPrevNext {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevNext))
		}
	})
	t.Run("riderless record does not compete", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveScopeCard(f.scope) // PrevScopePlayer|Combat: no rider, seat 0
		f.resolveBareAll(PlayerCode(0))
		bear := f.perm(f.bear, 1)
		f.bearSrc(bear)
		srcSl := f.slot(bear)
		if s.g.prevX[0] != 0 || !s.prevRecHit(f.k, 0, srcSl, f.bear, prevTgt{0, NoSlot}, true) {
			t.Fatal("precondition: the riderless scope record does not match the event")
		}
		at := f.bareShieldOn(PlayerCode(0), true)
		if at < 0 {
			t.Fatal("precondition: no bare unbounded shield matches the event")
		}
		if n, ok := s.prevNextTake(f.k, 1, srcSl, f.bear, PlayerCode(0), 0, NoSlot, true, 3); !ok || n != 0 {
			t.Fatalf("the bare shield did not prevent the event whole: dealt %d, applied %v", n, ok)
		}
		if got := s.StallReason(); got != 0 {
			t.Fatalf("stalled %s beside a riderless scope record", StallName(got))
		}
	})
}
