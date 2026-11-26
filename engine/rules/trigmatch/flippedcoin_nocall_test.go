package trigmatch

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// TestFlippedCoinSkipsUncalledFlip: an uncalled NoCall$ flip is neither won
// nor lost (CR 705.2; Forge FlipCoinEffect.flipCoin runs FlippedCoin only for
// a called or FlipCoinMod-fixed flip), so its result Note still counts as a
// flip but fires no "whenever you win a coin flip" line; a called heads does.
func TestFlippedCoinSkipsUncalledFlip(t *testing.T) {
	tr := cards.Trigger{Mode: "FlippedCoin", Params: map[string]string{"ValidResult": "Win"}}
	called := effects.FlipCoinNote(1, 0, true)
	uncalled := effects.FlipCoinNoCallNote(1, 0, true)
	if _, win, ok := effects.FlipNoteResult(uncalled); !ok || !win {
		t.Fatalf("an uncalled heads is no longer a flip Note: ok=%v win=%v", ok, win)
	}
	if !flippedCoinMatches(nil, tr, 1, called) {
		t.Error("a called win did not match ValidResult$ Win")
	}
	if flippedCoinMatches(nil, tr, 1, uncalled) {
		t.Error("an uncalled heads matched ValidResult$ Win")
	}
}
