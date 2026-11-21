package oracle

import "testing"

// dr-mbc-bbab pair 50 (seed 5044663500733291605): at a p0 priority decision
// after an Component Pouch activation and four passes, dirac offered
// cast(#32{Pelakka Wurm}) alongside cast(Arrow Storm) while gorge offered only
// pass and Arrow Storm — gorge's affordability refused a legal cast of
// Pelakka Wurm.
func TestMBCPelakkaWurmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_pelakka_cast_missing")
}
