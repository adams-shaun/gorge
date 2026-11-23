package oracle

import "testing"

// TestMBCCelestialGatekeeperReprosLockstep replays the recorded divergent
// game: Hazel's Brewmaster exiled Hua Tuo, so its Food token has Hua Tuo's
// "{T}: ... Activate only during your turn, before attackers are declared."
// (CR 602.5b). With attackers already declared the granted ability is not
// offered; gorge's battlefield grant walk once skipped the phase window and
// offered it at decision 342.
func TestMBCCelestialGatekeeperReprosLockstep(t *testing.T) {
	stdReproNamed(t, "granted_ability_activation_phases")
}
