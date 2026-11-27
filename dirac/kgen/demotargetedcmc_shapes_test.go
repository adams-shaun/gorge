package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoTargetedCmcShapes: Behemoth of Vault 0's ImmediateTrigger and
// Kozilek, the Great Distortion's AB$ Counter both read SVar:X
// Targeted$CardManaCost before the effect declares its one target. The read
// lowers to the pending slot (dirac/kgen/lower_targetmv.go):
//   - Behemoth: the dies trigger declares the nonland-permanent slot and opens
//     with UnlessSwitched / PayEnergy over MVOf(Target0);
//   - Kozilek: the activated ability's Discard<1/Card.cmcEQX> filter carries a
//     PredCMCLEV bound by ExMVOf(Target0), beside the Counter's target slot.
func TestDemoTargetedCmcShapes(t *testing.T) {
	names := []string{"Behemoth of Vault 0", "Kozilek, the Great Distortion"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	targetMV := func(v kop.ValueRef) bool {
		if v.K != kop.VExpr || int(v.N) < 0 || int(v.N) >= len(pt.exprs) {
			return false
		}
		ex := &pt.exprs[v.N]
		return ex.Kind == kop.ExMVOf && ex.Ref == kop.RefTarget0
	}
	seen := map[string]bool{}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		switch b.def.Name {
		case "Behemoth of Vault 0":
			for _, ab := range b.def.Abilities {
				if len(ab.Targets) == 0 {
					continue
				}
				seen["behemoth target"] = true
				for _, in := range ab.Prog {
					if in.Op == kop.OpUnlessSwitched && in.C&0xF == kop.PayEnergy && targetMV(in.A) {
						seen["behemoth MVOf(Target0)"] = true
					}
				}
			}
		case "Kozilek, the Great Distortion":
			ab := b.def.Abilities[0]
			if len(ab.Targets) == 0 || ab.Cost.Flags&kop.CostDiscardFiltered == 0 {
				t.Fatalf("Kozilek: targets %d, discard-filtered flag %v", len(ab.Targets), ab.Cost.Flags&kop.CostDiscardFiltered != 0)
			}
			for f := ab.Cost.Filter; f != 0; f = pt.filters[f].Or {
				fl := &pt.filters[f]
				if fl.Pred == kop.PredCMCLEV && targetMV(fl.PredV) {
					seen["kozilek cost MVOf(Target0)"] = true
				}
			}
		}
	}
	for _, k := range []string{"behemoth target", "behemoth MVOf(Target0)", "kozilek cost MVOf(Target0)"} {
		if !seen[k] {
			t.Errorf("%s did not lower", k)
		}
	}
}
