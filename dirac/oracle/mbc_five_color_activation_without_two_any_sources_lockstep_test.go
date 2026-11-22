package oracle

import "testing"

// CR 601.2h / 605.3: Go-Shintai of Life's Origin's {W}{U}{B}{R}{G} activation
// beside Pillarverge Pathway (W), a Swamp, Akoum Refuge, two Forests, School of
// the Unseen ({2},{T}: any colour) and a summoning-sick Eldrazi Scion. The kernel
// offers the activation (School's U fee paid by a Forest and the Scion's
// sacrifice) and gorge does not (decision 621 of 621, pool tail-42-c000 pair 37).
// Which engine is wrong is not settled; see .ds4/scratch/DIAGNOSIS.md.
func TestMBCGoShintaiOfLifeSOriginReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "five_color_activation_without_two_any_sources")
}
