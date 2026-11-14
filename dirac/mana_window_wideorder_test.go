package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestScriptEngineAnswerWideOrder pins the script search's answer order on a
// wide two-unit any-colour ask (Interdimensional Web Watch): the ask's
// options each carry a single ManaSymbol (mana_activation.go lays the two
// units out unit-major), so gorge's scriptOrderAnswers reorders its
// colour-pair answers need-first exactly as it does a single-colour ask, and
// scriptEngineAnswer must translate the search's index back to engine order
// (wideAnswer's multisets). The pass-through that left the pairs in engine
// order made the search try W W, W U, W B, W R before any pair naming the
// announced colour -- the order mana_mixgaps.go's wide walk already mirrors.
func TestScriptEngineAnswerWideOrder(t *testing.T) {
	sc, id, _ := kwrScen(t)
	ww, ok := id["Interdimensional Web Watch"]
	if !ok {
		t.Fatal("Interdimensional Web Watch is not in the wascastspell table")
	}
	ab := sc.k.gen.anyR(ww)
	if ab < 0 || !restrWideAnyMana(&sc.k.t.Cards[ww].Abilities[ab]) {
		t.Fatalf("precondition: Web Watch's anyR ability %d is not the wide two-unit gap", ab)
	}
	sl := sc.slot(sc.perm(ww, 0))
	sent, ok := id["Sentinel of Lost Lore"]
	if !ok {
		t.Fatal("Sentinel of Lost Lore is not in the wascastspell table")
	}
	o := sc.hand(sent, 0)
	a := MakeAction(AkCast, TCode(o), 0, 0)
	need := sc.s.playNeed(sc.k, a)
	if !need[kop.G] || need[kop.W] {
		t.Fatalf("precondition: the cast's announced colours %v, want {G}", need)
	}
	var anyR slotMask
	anyR = anyR.with(sl)
	// gorge's scriptOrderAnswers on the ask: the G-naming pairs first in
	// engine order (W G, U G, B G, R G, G G), then the rest, engine order
	// kept inside each half.
	want := []int{4, 8, 11, 13, 14, 0, 1, 2, 3, 5, 6, 7, 9, 10, 12}
	for c := 0; c < wideAnswers; c++ {
		if got := sc.s.scriptEngineAnswer(sc.k, a, sl, anyR, slotMask{}, c); got != want[c] {
			t.Fatalf("search answer %d maps to engine answer %d, want %d", c, got, want[c])
		}
	}
	if ca, cb := wideAnswer(want[0]); ca != 0 || cb != kop.G {
		t.Fatalf("the first answer decodes (%d, %d), want the (W, G) pair", ca, cb)
	}
}

// TestWideEngineAnswerEmptyNeed: with nothing announced the pairs keep
// engine order (gorge's scriptOrderAnswers returns the answers unchanged).
func TestWideEngineAnswerEmptyNeed(t *testing.T) {
	var need [kop.NumManaKinds]bool
	for c := 0; c < wideAnswers; c++ {
		if got := wideEngineAnswer(need, c); got != c {
			t.Fatalf("no need: search answer %d maps to engine answer %d, want %d", c, got, c)
		}
	}
}
