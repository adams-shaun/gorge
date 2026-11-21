package oracle

import "testing"

// gorge and dirac part at an `options options` priority decision: only the
// kernel offers activate(Tramway Station, ability 1) -- the {2}{B}{R}, {T},
// sacrifice-self draw. It is affordable (CR 601.2g, 605.3a): the seat's
// Diamond Kaleidoscope untaps nothing but its "Sacrifice a Prism token: Add
// one mana of any color" needs no tap, so the two Prism tokens on the
// battlefield fund the {B} and {R} pips while two untapped basics pay the
// {2} and the untapped Tramway Station pays its own {T} and self-sacrifice.
// gorge's payment planner prices a mana ability whose cost sacrifices
// ANOTHER permanent as TierDeferred (engine/rules/pay/planshape.go's
// self-sacrifice-only last-resort branch), so the planner builds no witness
// and the activation is withheld; the kernel offers it.
func TestMBCTramwayStationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_other_mana_source")
}
