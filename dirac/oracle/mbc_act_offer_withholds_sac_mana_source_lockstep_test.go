package oracle

import (
	"testing"
)

// Hearth Kami's "{X}, Sacrifice Hearth Kami: Destroy target artifact with
// mana value X" beside Ashnod's Altar's "Sacrifice a creature: Add {C}{C}" and
// a Crumbling Necropolis: gorge offers the activation (X-bound sweep, decision
// 307 of pool tail-71, surface autopay); the kernel offered pass only, because
// its potential census refused the printed sacrifice-mana ability (its cost is
// no bare tap, a {T}-shaped source, and the tap-another search prices it only
// at payment).
func TestMBCHearthKamiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_offer_withholds_sac_mana_source")
}
