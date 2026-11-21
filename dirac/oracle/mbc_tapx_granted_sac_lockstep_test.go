package oracle

import "testing"

// CR 602.2 with CR 118.3: Seton, Krosan Protector ("Tap an untapped Druid you
// control: Add {G}") and a Food token under Ninja Pizza ("{T}, Sacrifice this
// artifact: Add one mana of any color") are two mana sources beside Urborg
// Volcano and a Swamp, so Tyrant's Machine's {4}, {T} is payable and gorge
// offers its activation (decision 823 of tail-53-c000 pair 121). The kernel
// left the activation out.
func TestMBCTyrantSMachineReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_another_beside_granted_sac_food")
}
