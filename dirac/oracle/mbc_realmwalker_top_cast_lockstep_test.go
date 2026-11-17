package oracle

import "testing"

// Realmwalker ("you may cast creature spells of the chosen type from the top
// of your library") put onto the battlefield by Zirilan of the Claw chose Elf,
// and an Elf, Nurturer Initiate, lay on top after Zirilan's shuffle. Gorge
// offered the cast; the kernel's may-play filter read no source choice and
// offered nothing (decision 368).
func TestMBCRealmwalkerTopCastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "may_play_top_reads_sources_chosen_type")
}
