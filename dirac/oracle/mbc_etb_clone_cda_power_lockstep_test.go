package oracle

import (
	"testing"
)

// CR 613.3h with CR 613.4d and CR 208.2a: Kolaghan Forerunners's
// characteristic-defining power (one creature you control, itself) plus three
// +1/+1 counters (Ordeal of Purphoros) is 4, so when its controller's
// Deceptive Frostkite would enter, the copy election's "Creature.YouCtrl+
// powerGE4" selector admits the Forerunners and the choice must offer it
// beside the decline. The gorge side read the CDA star as 0 and priced the
// Forerunners 3 (counters alone): decision 599 offered no clone at all.
func TestMBCKolaghanForerunnersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "etb_clone_cda_power")
}
