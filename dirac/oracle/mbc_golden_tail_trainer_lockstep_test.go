package oracle

import "testing"

// Worldsoul Colossus's {X}{G}{W} cast with three convoked creatures. The
// announced picks pay its {G} and two generic, so the X must be at least 2 for
// every pick to reduce something, and the remaining {W} has to come from the
// pool. The convoke picks are tournament-fixed before X is announced
// (CR 601.2b), so the X ask may only offer values the picks and the pool
// actually pay (CR 601.2g): with the only white source among the picks
// (Sage of the Maze's "{T}: Add two mana in any combination of colors"), no X
// is payable and both engines fall back to the old offer (X = 0) before the
// mana window reverses the cast (CR 733.1). The kernel kept the picked Sage's
// any-colour units in the X census, so it offered X = 2 and X = 3 off the same
// tap the convoke pick already spent (decision 1717 of tail-36 pair 25).
func TestMBCGoldenTailTrainerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "golden_tail_trainer")
}
