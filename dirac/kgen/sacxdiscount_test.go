package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// sacDiscountProbe is Dargo, the Shipwrecker's announced sacrifice and its
// X-reading self discount without the SacrificedThisTurn tally (the part
// the kernel's State has no tally for): the Sac<X> description holds a
// slash ("and/or"), the self ReduceCost carries Relative$ True, and the
// permanent spell has AdditionalDesc$ text.
const sacDiscountProbe = "Name:Sac Discount Probe\nManaCost:6 R\nTypes:Creature Giant Pirate\nPT:7/5\n" +
	"A:SP$ PermanentCreature | Cost$ 6 R Sac<X/Artifact;Creature/any number of artifacts and/or creatures> | AdditionalDesc$ This spell costs {2} less.\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ Y | EffectZone$ All | Relative$ True\n" +
	"SVar:X:Count$xPaid\nSVar:Y:SVar$X/Times.2\n"

// TestXMinSacXCarriersDargo: a Sac<X/Spec/prompt> whose prompt holds a slash
// lowers to the announced sacrifice (CostSac, N 0), and the self discount's
// Relative$ and the permanent spell's AdditionalDesc$ read as text, so
// Dargo's only refusal left is the SacrificedThisTurn tally.
func TestXMinSacXCarriersDargo(t *testing.T) {
	d, rs := kwardpowBuild(t, "Sac Discount Probe", sacDiscountProbe)
	if all := kpaylifexReasons(rs); all != "" {
		t.Errorf("Sac Discount Probe: refusals %q", all)
	}
	if d.SpellCost.Flags&kop.CostSac == 0 || d.SpellCost.N != 0 {
		t.Errorf("Sac Discount Probe: want SpellCost CostSac with N 0, got %+v", d.SpellCost)
	}

	// the corpus card: exactly the tally refusal
	_, all := kxminsacxCard(t, "Dargo, the Shipwrecker", "d", "dargo_the_shipwrecker.txt")
	if !strings.Contains(all, "SacrificedThisTurn Artifact,Creature") {
		t.Errorf("Dargo: refusals %q, want the SacrificedThisTurn tally refusal", all)
	}
	for _, gone := range []string{"cost token", "Relative", "AdditionalDesc"} {
		if strings.Contains(all, gone) {
			t.Errorf("Dargo: refusals %q still name %q", all, gone)
		}
	}
}
