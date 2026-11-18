package oracle

import "testing"

// CR 509.1c with CR 509.1b: with a lure in combat (Nemesis Mask's "All
// creatures able to block CARDNAME do so.") every creature able to block the
// lure attacker is bound to it, so a duty blocker's pair against a different
// attacker sits in no legal declaration while the lure attackers can host
// every duty blocker -- and a duty blocker's pair against a menace attacker
// beside it does not meet the floor the blockers outside the duty can field.
// gorge offered both (Shaper Parasite and an Insect token over Dauntless
// Cathar's lure onto Rograkh, Son of Rohgahh), the kernel its lure blocks
// only (dr-mbc-baab pair 162, decision 587 of 587).
func TestMBCShaperParasiteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_duty_blocks_nonlure_pairs")
}
