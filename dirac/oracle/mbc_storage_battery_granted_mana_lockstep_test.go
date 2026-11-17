package oracle

import "testing"

// CR 602.2b, 601.2g-h, 605.1a: p1 holds a Library (a land with "{T}: Add {U}
// or {R}" and "{4}, {T}: Investigate") beside a Joiner Adept ("Lands you
// control have '{T}: Add one mana of any color'"), an Island, a Forest and a
// Dwarven Hold holding one storage counter. The Library's granted ability and
// its own {T} are one tap, and so are the Dwarven Hold's granted ability and
// its storage battery: Island, Forest and one of the Hold's two faces make
// three mana, never the four the Investigate activation costs (decision 342
// of tail-08 focus pair 96).
func TestMBCStorageBatteryGrantedManaLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_and_granted_mana_share_one_tap")
}
