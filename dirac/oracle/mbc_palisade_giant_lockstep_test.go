package oracle

import (
	"testing"
)

// CR 509.1a-c: a blocker's spare slot (Brave the Sands raises each of p1's
// creatures' block count) may still cover an attacker its group owes, even when
// another blocker in the same group already blocks that attacker and menace
// (Seasoned Dungeoneer's Skeleton Token) restricts only that attacker's own
// blockers. At decision 414 the oracle's emulation built its set of still-free
// blockers from blockers in no chosen pair, dropping a declared blocker's spare
// slot, and so refused three of gorge's five legal picks and reported a false
// divergence (dr-mbc pair 36 of tail-56, seed 5357339939419112416, Palisade
// Giant holding the spare slot).
func TestMBCPalisadeGiantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "palisade_giant_menace")
}
