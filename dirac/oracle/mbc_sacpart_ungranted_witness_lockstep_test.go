package oracle

import "testing"

// CR 601.2g-h with CR 118.8: Ichor Explosion ({5}{B}{B} plus sacrificing a
// creature) beside Elven Chorus (creatures gain "{T}: Add one mana of any
// color") and two Eldrazi Spawn tokens (sacrifice: add {C}). The printed
// sources hold exactly seven mana, but the part's candidate must be held
// back from it (gorge planReserving), so the witness is six mana and the
// cast is priced only through a prefix script; the autopay surface leaves a
// script-only plain cast out. The kernel offered it: its witness mirror
// priced the part's reservation against the full availability, granted mana
// included, instead of the planner's census. dr-mbc-tail-51 pair 18, seed
// 732850 (the evidence loop's own game; Immoral Bargain is the card the
// loop owed evidence, Ichor Explosion the one the divergence implicates).
func TestMBCIchorExplosionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacpart_ungranted_witness")
}
