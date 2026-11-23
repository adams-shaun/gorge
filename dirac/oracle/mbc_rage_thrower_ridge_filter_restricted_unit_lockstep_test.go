package oracle

import (
	"testing"
)

// CR 601.2g-h with CR 605.3a: the same gap skyshroud_vampire_priority_cast_offer
// pins, on its second instance. Evidence-loop pair (synth tail-73, seed base
// 340771, pair 6, gorge leads): p0's lands at the recorded decision 377 are two
// Mountains, three Forests and a just-played Shadowblood Ridge
// ("{1}, {T}: Add {B}{R}") beside Untaidake, the Cloud Keeper ("{T}, Pay 2
// life: Add {C}{C}", spend only on legendary spells) — a restricted planner
// unit whose {C}{C} cannot fund Rage Thrower {5}{R}. The fifth and sixth mana
// are exactly the Ridge's filter route (pay its {1} from a tapped Forest, tap
// it, keep a Mountain's {R} for the pip), so gorge offered cast(Rage Thrower);
// the kernel gated the route off beside the restricted unit and withheld it.
// The fix (censusGapMana leaves restricted planner units to the census, so a
// filter beside one is filter-only again) makes this recording replay clean.
func TestMBCRageThrowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rage_thrower_ridge_filter_restricted_unit")
}
