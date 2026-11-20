package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBareWasCastFromYourHandScopesToController mirrors gorge's
// TestBareWasCastFromYourHandScopesToTriggerController (engine/rules): a
// SpellCast line with the bare wasCastFromYourHand and no
// ValidActivatingPlayer$ (Baron Helmut Zemo) means the trigger controller's
// own hand (CR 109.5), so it lowers to the kernel's cast-from-hand-by-the-
// controller flag (Tf2CastFromHand), never the any-caster PredWasCastSpell. The
// negated spelling has no carrier and refuses by name.
func TestBareWasCastFromYourHandScopesToController(t *testing.T) {
	const body = "SVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1"
	why, ab, _ := ktqBuildSV(t, "T:Mode$ SpellCast | ValidCard$ Card.Black+wasCastFromYourHand | TriggerZones$ Battlefield | Execute$ ProbeDraw", body)
	if why != "" {
		t.Fatalf("unscoped bare word refused: %q", why)
	}
	if ab.Trig.Flags2&kop.Tf2CastFromHand == 0 {
		t.Errorf("flags2 %#x, want Tf2CastFromHand (the controller's hand)", ab.Trig.Flags2)
	}
	why, _, _ = ktqBuildSV(t, "T:Mode$ SpellCast | ValidCard$ Card.!wasCastFromYourHand | TriggerZones$ Battlefield | Execute$ ProbeDraw", body)
	if !strings.Contains(why, "without ValidActivatingPlayer") {
		t.Errorf("unscoped negated word: refusal %q, want the named one", why)
	}
}
