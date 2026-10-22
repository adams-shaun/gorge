package netsimd_test

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
)

func TestLegacyNetIgnoresRelationalSlots(t *testing.T) {
	recs := corpus(t)
	g := loadSIMD(t, 0)
	t.Cleanup(func() { netsimd.ForceScalar = false })
	if got := g.RawWidth(); got != policynet.EntityRawWidthV1 {
		t.Fatalf("g7 raw width %d, want the format-1 width %d", got, policynet.EntityRawWidthV1)
	}
	withRel := 0
	for _, forceScalar := range []bool{false, true} {
		netsimd.ForceScalar = forceScalar
		var sc netsimd.Scratch
		for i := range recs {
			r := &recs[i]
			zeroed := r.st
			zeroed.Cards = make([]policynet.EntityCard, len(r.st.Cards))
			for k := range r.st.Cards {
				zeroed.Cards[k] = r.st.Cards[k]
				zeroed.Cards[k].Raw = append([]float32(nil), r.st.Cards[k].Raw...)
				clear(zeroed.Cards[k].Raw[policynet.EntityRawWidthV1:])
			}
			if forceScalar {
				rel := false
				for k := range r.st.Cards {
					for _, f := range r.st.Cards[k].Raw[policynet.EntityRawWidthV1:] {
						if f != 0 {
							rel = true
						}
					}
				}
				if rel {
					withRel++
				}
			}
			a := make([]float32, len(r.opts))
			b := make([]float32, len(r.opts))
			va := g.Eval(&sc, &r.st, r.opts, a)
			vb := g.Eval(&sc, &zeroed, r.opts, b)
			if math.Float32bits(va) != math.Float32bits(vb) {
				t.Fatalf("scalar=%v rec %d: value %v vs %v", forceScalar, i, va, vb)
			}
			for j := range a {
				if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
					t.Fatalf("scalar=%v rec %d opt %d: %v vs %v", forceScalar, i, j, a[j], b[j])
				}
			}
		}
	}
	if withRel < 1 {
		t.Fatalf("only %d corpus records carry a non-zero relational slot; the check proves nothing", withRel)
	}
	t.Logf("records with relational slots: %d of %d", withRel, len(recs))
}
