package oracle

import "testing"

// CR 106.1, 601.2f: Incubation Druid (a +1/+1 counter on it) adds three mana of
// one type any land its controller controls could produce, so with a Forest it
// pays {2}{G}{G} (Creeping Corrosion) and with a Swamp it pays {4}{B}{B}
// (Vengeant Vampire). The two engines parted at a priority decision where only
// the kernel offered both double-coloured casts (tail-55, pair 24).
func TestMBCCreepingCorrosionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_source_double_colour_cast")
}
