package oracle

import "testing"

// CR 605.3a, 601.2g-h: gorge's relaxed proof takes the board's paid mana
// filters a subset at a time, so Henge of Ramos's "{2},{T}: one mana of any
// color" (one mana for two) does not keep a Simic Signet's "{1},{T}: {G}{U}"
// from paying a cost the Signet and Henge's free {C} reach. The kernel ran one
// proof with every filter in it and dropped the play: Ketria Triome's cycling,
// Otawara's channel and Resounding Silence's cycling were each offered by
// gorge alone (tail-61 pairs 13, 165 and 11). The three recordings are the
// channel, an adapt-style fee and a cycling fee on such boards.
func TestMBCKetriaTriomeReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"filter_net_negative_beside_first_only",
		"filter_net_negative_adapt_fee",
		"filter_net_negative_cycling_fee",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
