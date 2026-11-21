package oracle

import "testing"

// CR 106.6, 605.3a with CR 601.2g: p1's Automated Artificer ("{T}: Add {C}.
// Spend this mana only to activate an ability or cast an artifact spell") is
// a restricted planner unit, and Galazeth Prismari gives every artifact an
// any-colour "{T}: Add one mana of any color" that only an instant or sorcery
// spell may spend, so the Artificer also holds a granted ability this
// activation refuses. Beside Shadowblood Ridge ("{1}, {T}: Add {B}{R}") with
// every other land tapped for Thunderous Wrath, Mindshrieker's "{2}"
// activation is paid by the Artificer's {C} (it pays the Ridge's fee, an
// ability activation the restriction admits) and the Ridge's {B}{R}. gorge's
// census counts the Artificer's {C} per payment, so its relaxed proof pays
// the play and its prefix search prices the chain. The kernel's grantCap
// re-read the relaxed census with the refused grant dropped
// (relaxRestrRefuses) and left the Artificer's own admitted {C} out of it, so
// it proved the play insufficient and never searched it. The fix keeps the
// restricted planner units the payment may spend in that read.
func TestMBCMindshriekerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restr_unit_fee_chain_beside_refused_grant")
}
