package oracle

import "testing"

const lochmereSerpentFilterSacRepro = "lochmere_serpent_sac_cost_beside_filter_mana_unmodelled"

// Lochmere Serpent's "{B}, Sacrifice a Swamp: You gain 1 life and draw a card"
// was the cause. With only a tapped Xander's Lounge as the Swamp, p1 pays the {B}
// through a chain: Azorius Chancery's {W}{U} pays Cabal Coffers' {2}, and
// Coffers' {B} per Swamp (the Lounge, still on the battlefield) pays the
// activation, which then sacrifices the Lounge (CR 601.2g-h, 605.3b). The
// first-only filter's admission bound once read the all-at-once land as one
// mana, so Coffers was never admitted and only the {U}, sacrifice an Island
// activation was offered (decision 821 of tail-41-c000). With the bound
// counting the whole production (dirac/mana_filterfirst.go filtOtherMana, CR
// 605.3b, 605.1a) Coffs is admitted, both engines offer the {B} activation,
// and the recorded game replays clean; the card is therefore no longer
// refused (dirac/support/unrefused.tsv), and this test pins the unrefusal's
// proof.
func TestMBCLochmereSerpentReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, lochmereSerpentFilterSacRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", lochmereSerpentFilterSacRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", lochmereSerpentFilterSacRepro, res.Stall)
	}
}
