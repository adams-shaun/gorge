package oracle

import "testing"

// Mirror Room's token copy of a manifested creature is a nameless 2/2 with no
// colour or abilities, and it is face up (CR 707.2).
func TestTokenCopyOfFaceDownPermanent(t *testing.T) {
	synthPairsReaching(t, "std-dsk", Config{GorgeAutopaySurface: true}, []int{6, 7}, onBattlefield("Mirror Room"))
}
