package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGatedDrawMoreStillAsksTheOrder: only the plain, ungated "draw N cards
// instead" shape is commutative. Reed Richards (FirstExtraCardDrawnThisTurn$,
// draw four) beside Thought Reflection (draw two) composes to 8 cards in one
// CR 616.1 order and 5 in the other, so the affected player keeps the order
// ask whichever replacement the battlefield scan meets first.
func TestGatedDrawMoreStillAsksTheOrder(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hands := map[string]int{}
	for _, reedFirst := range []bool{true, false} {
		for _, pickReed := range []bool{true, false} {
			e := drawReplEngine(t, 744)
			var reed, tr state.ObjID
			add := func(reedNow bool) {
				if reedNow {
					reed = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Reed Richards, Smartest Man"))
				} else {
					tr = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Thought Reflection"))
				}
			}
			add(reedFirst)
			add(!reedFirst)
			setDrawHand(t, e, 0, 0)
			e.pending = nil
			emitDraw(t, e, 0)
			d := e.Pending()
			if d == nil || d.Kind != decision.KReplacement {
				t.Fatalf("reedFirst=%v: a gated draw-more competition must park the CR 616.1 order ask, got %+v", reedFirst, d)
			}
			ri, ti := indexOfObjOption(d, reed), indexOfObjOption(d, tr)
			if ri < 0 || ti < 0 {
				t.Fatalf("reedFirst=%v: ask must offer both replacements, got %+v", reedFirst, d.Options)
			}
			if pickReed {
				submitChoices(t, e, ri)
			} else {
				submitChoices(t, e, ti)
			}
			key := "tr"
			if pickReed {
				key = "reed"
			}
			if prev, ok := hands[key]; ok && prev != len(e.G.Zone(state.ZHand, 0)) {
				t.Fatalf("pick %s: hand %d vs %d depending on add order; the ask must make the order the player's", key, prev, len(e.G.Zone(state.ZHand, 0)))
			}
			hands[key] = len(e.G.Zone(state.ZHand, 0))
		}
	}
	if hands["reed"] == hands["tr"] {
		t.Fatalf("precondition: the two orders must differ in cards drawn, both gave %d", hands["reed"])
	}
}

// TestTwoNotionThievesStillAskTheOrder: Notion Thief's body draws for its
// controller, not the drawing player, so two of them are not a plain
// draw-more competition and the affected player still orders them.
func TestTwoNotionThievesStillAskTheOrder(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 745)
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Notion Thief"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Notion Thief"))
	setDrawHand(t, e, 0, 1)
	e.pending = nil
	emitDraw(t, e, 0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("two Notion Thieves must park the CR 616.1 order ask, got %+v", d)
	}
}
