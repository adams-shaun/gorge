package oracle

import "testing"

// CR 605.1b with CR 601.2b: a copied Halo Forager's "you may pay {X}" announces X
// beside Mana Flare, which adds a mana for every land tapped. Four untapped
// lands pay eight, so X runs 0..8; gorge's source census priced the lands
// without the tap trigger's mana (0..4) and dirac's potential walk did the same
// (0..5), where its per-source census counted eight (decision 466 of
// tail-41-c000 pair 7, Multiversal Recruitment's token copy).
func TestMBCMultiversalRecruitmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_x_tap_extra")
}
