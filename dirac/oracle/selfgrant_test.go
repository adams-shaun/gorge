package oracle

import "testing"

// Ghired's grant reaches Ghired too: gorge offers the activation whichever
// permanent granted it, and so does the vocabulary.
func TestSelfGrantedAbilityOffered(t *testing.T) {
	synthPairsReaching(t, "std-otj", Config{GorgeAutopaySurface: true}, []int{14, 43}, anyOf(onBattlefield("Ghired, Mirror of the Wilds"), onBattlefield("Ghired, Conclave Exile")))
}
