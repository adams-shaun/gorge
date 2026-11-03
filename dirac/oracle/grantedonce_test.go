package oracle

import "testing"

// TestGrantedOnceLockstep plays the std-fdn pairs where Dropkick Bomber is
// activated more than once on one Goblin: each grant is an instance of the line
// and all of them trigger on the combat damage (CR 113.2c).
func TestGrantedOnceLockstep(t *testing.T) {
	synthPairsReaching(t, "std-fdn", Config{GorgeAutopaySurface: true}, []int{207, 1467}, onBattlefield("Dropkick Bomber"))
}
