package oracle

import "testing"

// CR 106.6, 605.3b: a Powerstone's {C} can't be spent on a nonartifact spell
// but pays a mana ability's activation cost, so Powerstone -> Hall of Tagsin
// ({1},{T}: add one mana of any colour) beside three Islands and Accursed
// Duneyard pays Believe's {4}{G} from the graveyard (aftermath, Reason //
// Believe; decision 424 of tail-39-c000 pair 3), and Invent's {4}{U}{R} from
// the hand. A face cast's filter prefix search keeps the restricted covered
// source as funding for a census gap's fee, as gorge's coveredSpendable does
// (mana_filter.go gapFeeSpendable), so both engines offer the cast.
func TestMBCReasonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "split_card_castface_offer")
}
