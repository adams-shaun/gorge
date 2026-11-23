package oracle

import "testing"

// CR 106.6 with CR 602.2: Purple Pentapus's graveyard ability ({2}{B}, tap an
// untapped creature you control) is paid by a Swamp's {B} and Karn, Legacy
// Reforged's floating {C}{C}, whose "can't cast nonartifact spells" restriction
// bars only spells; Karn is the creature tapped. The kernel priced the tapXType
// part's reservation from the printed census alone and left the floating
// restricted batch out, so it withheld the activation gorge offered (decision
// 750 of ev-tail-62-c000 pair 54).
func TestMBCPurplePentapusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_pool_pays_tapx_graveyard_ability")
}
