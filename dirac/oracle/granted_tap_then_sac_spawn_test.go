package oracle

import "testing"

// TestGrantedTapThenSacSpawnReprosLockstep: Path of Annihilation grants every
// Eldrazi "{T}: Add one mana of any color", so an Eldrazi Spawn token taps for
// one mana and is then sacrificed for another (CR 605.1a: both are mana
// abilities; the sacrifice-only one needs no {T}). With a Mountain and two such
// Spawns untapped, gorge offers Court of Embereth's {2}{R}{R}: the granted taps
// float RR and the Mountain and the two sacrifices pay the {2}.
func TestGrantedTapThenSacSpawnReprosLockstep(t *testing.T) {
	stdReprosLockstep(t, "dr-mbc-babb", []stdRepro{
		{"granted_tap_then_sac_spawn", false, false}, // CR 605.1a: a tapped Spawn still sacrifices for {C}
	})
}
