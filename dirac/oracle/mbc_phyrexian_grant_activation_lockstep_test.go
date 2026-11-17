package oracle

import "testing"

// CR 107.4f, CR 119.4, CR 602.2b: Trespassing Souleater's "{U/P}: can't be
// blocked this turn" with its controller at 1 life and Chromatic Lantern tapped.
// The only blue left is a Mountain's granted any-colour ability. The kernel once
// offered it while gorge could not make the play (decision 392 of tail-72 pair
// 14); since the potential witness funds the pips a short life total cannot pay
// with their colour (phyrexian_activation_low_life_granted_mana_sacrificed),
// gorge makes the play through the script/grant route, so both engines offer the
// activation and the recording replays clean. The gate that withheld it under
// gorge's surface (phyActUngrantedOK) is gone.
func TestMBCTrespassingSouleaterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_granted_only_unlowerable")
}
