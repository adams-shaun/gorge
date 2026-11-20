package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// blgMaskRef is blgMask's per-type form, before it asked the layer walk once:
// one hasSubGen per basic type.
func blgMaskRef(s *State, k *prep, sl Slot) (m uint8) {
	for c, i := range k.gen.blgSub {
		if i != subOutside && s.hasSubGen(k, sl, i) {
			m |= 1 << c
		}
	}
	return m
}

// TestBlgMaskWalkOnce: blgMask answers from one layer walk where it took five,
// and must agree with the per-type hasSubGen it replaced on every land slot of
// four-seat edh games (the pods the bench plays), on every step.
func TestBlgMaskWalkOnce(t *testing.T) {
	pods := edhPods(t, 8)
	var s State
	var d Decision
	checked, nonzero, walked := 0, 0, 0
	for g, p := range pods {
		func() {
			defer func() { _ = recover() }() // a random-policy edh game can panic the kernel (BenchmarkTransitionEDH4)
			p.init(&s, uint64(g)+1)
			pol := NewRandomPolicy(uint64(g) + 1)
			for i := 0; i < 800; i++ {
				s.StepInto(&d)
				if d.Kind == DkNone {
					return
				}
				if k := s.k(); k.gen != nil && k.gen.blgOn {
					for m := s.occ; m.any(); m = m.rest() {
						sl := m.first()
						if s.typesAt(k, sl)&kop.TLand == 0 || s.genLost(k, sl) && !s.landSetLost(k, sl) {
							continue
						}
						ref := blgMaskRef(&s, k, sl)
						if got := uint8(s.blgMask(k, sl)); got != ref {
							t.Fatalf("pod %d step %d slot %d: blgMask %05b, per-type hasSubGen %05b", g, i, sl, got, ref)
						}
						checked++
						if ref != 0 {
							nonzero++
						}
						if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
							walked++
						}
					}
				}
				s.Apply(pol.Pick(&d))
			}
		}()
	}
	t.Logf("compared %d land slots (%d with a granted type, %d on the walk branch)", checked, nonzero, walked)
	if checked == 0 {
		t.Skip("no edh pod in the sample has a basic land type grant")
	}
}
