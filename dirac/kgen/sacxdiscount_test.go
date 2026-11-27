package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// sacDiscountProbe is Dargo, the Shipwrecker's announced sacrifice and its
// X-reading self discount: the Sac<X> description holds a slash ("and/or"),
// the self ReduceCost carries Relative$ True, and the permanent spell has
// AdditionalDesc$ text.
const sacDiscountProbe = "Name:Sac Discount Probe\nManaCost:6 R\nTypes:Creature Giant Pirate\nPT:7/5\n" +
	"A:SP$ PermanentCreature | Cost$ 6 R Sac<X/Artifact;Creature/any number of artifacts and/or creatures> | AdditionalDesc$ This spell costs {2} less.\n" +
	"S:Mode$ ReduceCost | Type$ Spell | Relative$ True | ValidCard$ Card.Self | Amount$ Y | EffectZone$ All\n" +
	"SVar:X:Count$xPaid\nSVar:Y:SVar$X/Times.2\n"

// TestXMinSacXCarriersDargo: a Sac<X/Spec/prompt> whose prompt holds a slash
// lowers to the announced sacrifice (CostSac, N 0), and the self discount's
// Relative$ and the permanent spell's AdditionalDesc$ read as text. Dargo's
// second ReduceCost reads the SacrificedThisTurn filter, so the corpus card
// lowers with no refusal.
func TestXMinSacXCarriersDargo(t *testing.T) {
	d, rs := kwardpowBuild(t, "Sac Discount Probe", sacDiscountProbe)
	if all := kpaylifexReasons(rs); all != "" {
		t.Errorf("Sac Discount Probe: refusals %q", all)
	}
	if d.SpellCost.Flags&kop.CostSac == 0 || d.SpellCost.N != 0 {
		t.Errorf("Sac Discount Probe: want SpellCost CostSac with N 0, got %+v", d.SpellCost)
	}

	// the corpus card: the value is a filter now, so nothing refuses
	_, all := kxminsacxCard(t, "Dargo, the Shipwrecker", "d", "dargo_the_shipwrecker.txt")
	if all != "" {
		t.Errorf("Dargo: refusals %q, want none", all)
	}
}
