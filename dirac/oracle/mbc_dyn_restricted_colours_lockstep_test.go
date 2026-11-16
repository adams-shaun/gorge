package oracle

import "testing"

// CR 106.1b with CR 106.6: Vhal, Candlekeep Researcher's {T} ability makes {C}
// equal to its toughness, which can't be spent on spells from hand. It makes
// no coloured mana, so beside an Island and a Holdout Settlement (one mana of
// any colour for a tapped creature) it can't pay Dauthi Embrace's {B}{B}. The
// kernel once counted the evaluated amount as that many mana of any one
// colour and offered the activation gorge withholds (decision 367 of the
// tail-42 autopay game, pair 2).
func TestMBCDauthiEmbraceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dyn_restricted_mana_keeps_its_colours")
}
