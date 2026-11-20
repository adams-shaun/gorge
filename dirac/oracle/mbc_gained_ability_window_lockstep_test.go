package oracle

import "testing"

// CR 602.5 / CR 113.3: an ability Necrotic Ooze has from a graveyard creature
// (Wu Longbowman: "Activate only during your turn, before attackers are
// declared") keeps that activation restriction. gorge's gained-ability offer
// skipped the window gate and offered it to the Ooze's controller in the
// opponent's combat-damage step (decision 518 of tail-61-c001 pair 9).
func TestMBCNecroticOozeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gained_ability_activation_window")
}
