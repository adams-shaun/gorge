package oracle

import "testing"

// TestActivationOptionsLockstep: an activated ability whose generic cost only
// every census gap together can pay is offered by the kernel exactly when gorge
// offers it. gorge's payment planner defers a depletion land (Saprazzan Skerry's
// "{T}, remove a depletion counter: Add {U}{U}"), so its mana is no census unit
// and one prefix search over every census gap in battlefield order prices the
// activation (CR 601.2g-h, 605.3b). The kernel's tap-another-permanent search
// (Birchlore Rangers) and its grant search (Gemhide Sliver's granted {T}) each
// left the depletion land out of their gaps, so the kernel withheld an
// activation gorge offered:
//   - Ebony Fly's {4} (two Plains, Rangers and the Skerry) and Valakut Invoker's
//     {8} beside Birchlore Rangers;
//   - Bloodrite Invoker's {8} and Dreadmalkin's {2}{B} with a sacrifice beside
//     Gemhide Sliver.
//
// Each recording is the first game the pair diverged in, played to the end with
// both engines in lockstep.
func TestActivationOptionsLockstep(t *testing.T) {
	for _, name := range []string{
		"depletion_gap_beside_tap_x_pays_activation",
		"depletion_gap_beside_tap_x_pays_eight_activation",
		"depletion_gap_beside_grant_pays_activation",
		"depletion_gap_beside_grant_pays_sacrifice_activation",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
