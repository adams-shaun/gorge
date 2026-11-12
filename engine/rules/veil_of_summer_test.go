package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestVeilOfSummerGrantsPlayerAndPermanents: Veil of Summer's "You and
// permanents you control gain hexproof from blue and from black until end of
// turn" (CR 702.11d) reaches the caster as well as their permanents.
func TestVeilOfSummerGrantsPlayerAndPermanents(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	bear := onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	veil := addHexproofCardToHand(t, e, 0, corpusCardText(t, "v/veil_of_summer.txt"))
	addMana(t, e, 0, "G")
	e.askPriority(0)
	castFirstOf(t, e, veil)
	resolveStack(t, e)
	t.Logf("bear keywords %v", e.Keywords(bear))
	t.Logf("player keywords %v", e.playerKeywords(0))
	for _, q := range []string{"Blue", "Black"} {
		if !e.hexproofQualityHeld(e.Keywords(bear), q) {
			t.Errorf("permanent lacks hexproof from %s", q)
		}
		if !e.hexproofQualityHeld(e.playerKeywords(0), q) {
			t.Errorf("player lacks hexproof from %s", q)
		}
	}
	_ = state.ZStack
}

func (e *Engine) hexproofQualityHeld(kws []string, q string) bool {
	for _, kw := range kws {
		if got, ok := hexproofQuality(kw); ok && got == q {
			return true
		}
	}
	return false
}
