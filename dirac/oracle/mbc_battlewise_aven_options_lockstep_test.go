package oracle

import "testing"

// Worldsoul Colossus's {X} announcement after a convoke pick: the picked Sage
// of the Maze is a Combo Any source ({T}: Add two mana in any combination of
// colors), and the kernel's X ceiling counted its units even though the pick
// taps it. Gorge offers X = 0,1 (the untapped Plains pay past {G}{W}); the
// kernel offered 0..3 (tail-36-c001 pair 15, decision 208, surface autopay).
func TestMBCBattlewiseAvenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "battlewise_aven_options")
}
