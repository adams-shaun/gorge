package oracle

import (
	"testing"
)

// Dreadwing's "{1}{U}{R}: +3/+0 and flying" beside Artifact Unknown Shores's
// "{1}, {T}: Add one mana of any color" filter (the only red source), three
// Islands, two Swamps and an Eldrazi Scion token: gorge offers the activation
// in its controller's upkeep (decision 418 of pair 308, pool tail-10, surface
// autopay); the kernel offered pass only.
func TestMBCDreadwingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_filter_fee_beside_sac_source")
}
