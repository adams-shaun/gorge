package oracle

import "testing"

// Ghired's grant reaches Ghired too: gorge offers the activation whichever
// permanent granted it, and so does the vocabulary.
func TestSelfGrantedAbilityOffered(t *testing.T) {
	synthPairs(t, "std-otj", []int{252, 404}, nil)
}
