package oracle

import (
	"testing"
)

// A fee-bearing mana ability with no {T} (Signpost Scarecrow's "{2}: Add one
// mana of any color") is one of gorge's potential-mana bound's units once the
// pool the other untapped sources float covers its fee (CR 601.2i: mana
// abilities may be activated while paying a spell; gorge rules/potential.go
// manaAbilityPayablePool), so a cast whose second blue pip only that ability
// makes is offered (Historian of Zhalfir's {2}{U}{U} beside one untapped
// Island, three Forests, a Sandstone Needle and the Scarecrow). The kernel's
// hand-cast census (dirac/mana.go canTapForManaPC) counts a source with a tap
// or sacrifice ability alone, so it drops the play and the options diverge at
// the priority decision (pool tail-45-c004, decision 777).
func TestMBCHistorianOfZhalfirReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_paid_gap_cast_offer")
}
