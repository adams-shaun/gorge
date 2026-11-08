package oracle

import "testing"

// The farm's std-fdn lockstep (FDN-s397, game xe:RAND:3:1) diverged at turn 14 on
// creature power and toughness with Dwynen, Gilt-Leaf Daen implicated: its static
// ability gives other Elf creatures you control +1/+1, beside Wilt-Leaf Liege's
// anthem. The pinned games reach Dwynen on the battlefield and must replay clean.
func TestPinnedFarmDwynenGiltLeafDaen(t *testing.T) {
	pinnedGames(t, "std-fdn", Config{GorgeAutopaySurface: true}, onBattlefield("Dwynen, Gilt-Leaf Daen"))
}
