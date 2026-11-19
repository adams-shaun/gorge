package oracle

import "testing"

// Cabaretti Initiate's {2}{R/W} activation beside its controller's Fetid
// Heath: the Heath's "{W/B}, {T}: Add {W}{W}, {W}{B}, or {B}{B}" is a
// hybrid-fee filter, and with no W or B mana anywhere (Mountains, a Forest,
// the Heath's plain {T}: Add {C}) its fee is unpayable, so the Heath can make
// no {W} for the pip (gorge's window pays a filter's fee through the ordinary
// mana payment, CR 605.3b) and the CR 601.2b pip ask offers {R/W}'s R face
// alone. The kernel once paid the {W/B} fee with any unit (a Mountain's {R})
// and priced the Heath's {W}{W} under it, offering the W face too (decision
// 384 of the recorded game; pool tail-01, surface autopay, seed 525781,
// pair 119).
func TestMBCCabarettiInitiateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_fee_colour")
}
