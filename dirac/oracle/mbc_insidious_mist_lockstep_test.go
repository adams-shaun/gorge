package oracle

import "testing"

// Elusive Tormentor's back face, Insidious Mist: "Whenever this attacks and
// isn't blocked, you may pay {2}{B}. If you do, transform it." gorge prices a
// trigger's own cost on its potential mana, which an untapped Firemind Vessel
// ("{T}: Add two mana of different colors", Produced$ Combo AnyDifferent) makes
// unbounded in every colour; dirac offered only the decline (decision 538 of
// pool tail-39 pair 21). Electing the pay then runs the manual window, which
// lists the Vessel in battlefield order and taps it for no mana ("unhandled
// Produced$ AnyDifferent"): beside an untapped Mountain the window taps the
// Mountain first and floats its {R} (decision 743 of the same game).
func TestMBCElusiveTormentorReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"insidious_mist_unblocked_pay_offer",
		"insidious_mist_pay_taps_lands_then_vessel",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
