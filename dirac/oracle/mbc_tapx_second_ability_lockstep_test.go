package oracle

import "testing"

// Bioessence Hydra ({3}{G}{U}) with Island, Firemind Vessel (two mana of
// different colors) and Survivors' Encampment ({T}: {C}, or {T} and tap an
// untapped creature: one mana of any color) untapped makes four mana at most
// (CR 605.3b, 106.1): the Encampment taps once. gorge offers no Hydra cast;
// the kernel did.
func TestMBCBioessenceHydraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapx_second_ability_beside_vessel")
}
