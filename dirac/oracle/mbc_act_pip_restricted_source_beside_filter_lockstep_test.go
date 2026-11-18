package oracle

import (
	"testing"
)

// Warden of the First Tree's "{3}{W/B}{W/B}{W/B}" activation beside
// Shadowblood Ridge's "{1},{T}: Add {B}{R}" filter and A-Base Camp's
// Warrior-only any colour (decision 382 of pair 11, pool tail-73, surface
// autopay): the Warden is a Warrior, so Base Camp's mana pays its ability
// and Swamp, Ridge and Base Camp make the third {B}; gorge announces the
// pip with both faces, the kernel's filter walk left the restricted source
// out and offered {W} only (CR 106.6, 601.2b).
func TestMBActPipRestrictedSourceBesideFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_pip_restricted_source_beside_filter")
}
