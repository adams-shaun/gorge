package oracle

import "testing"

// CR 106.1b with CR 106.6: Vhal, Candlekeep Researcher's {T} ability makes {C}
// equal to its toughness, which can't be spent on spells from hand. It makes
// no coloured mana, so beside an Island and a Holdout Settlement (one mana of
// any colour for a tapped creature) it can't pay Dauthi Embrace's {B}{B}. The
// kernel once counted the evaluated amount as that many mana of any one
// colour and offered the activation gorge withholds (decision 367 of the
// tail-42 autopay game, pair 2).
// The recording is pooled custom (the table its own decks need): the synthetic
// bucket it was found on re-cuts whenever the lowered card set moves.
//
// CR 605.3a with CR 605.3b: with one Swamp untapped, Dauthi Embrace's {B}{B}
// is payable only through School of the Unseen's {2},{T} filter, and gorge
// leaves it unpriced and unoffered: its 160-node prefix search spends the
// budget on funding lines before the two-Mountain one, because Karplusan
// Forest's mana ask lists {C}, {R} and {G} (the painful ability is no cost).
// The kernel funded with Karplusan's {C} alone, reached the paying line and
// offered the third activation (decision 1157 of the tail-42 full game, pair
// 97, seed 569004; act_filter_search_budget_pain_ask).
//
// CR 605.3a with CR 601.2h: the same Dauthi Embrace {B}{B}, with Holdout
// Settlement's tap-an-untapped-creature ask, School of the Unseen's filter and
// Karplusan Forest all untapped, outruns gorge's 160-node search; the kernel's
// filter search found the paying line and offered the activation (decision 385
// of the tail-42 autopay game, pair 11, seed 786007;
// dauthi_tapx_filter_budget).
func TestMBCDauthiEmbraceReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"dyn_restricted_mana_keeps_its_colours",
		"act_filter_search_budget_pain_ask",
		"dauthi_tapx_filter_budget",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
