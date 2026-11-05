package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwoDrawMoresSettleInScanOrder pins the CR 616.1 competition of two
// mandatory "draw N cards instead" replacements (two Mr. Spock, Science
// Officers on one battlefield): every candidate's complete replacement is
// "draw its N in the draw's place", so the composition draws N1·N2 cards
// from the library in the same order either way and the engine settles in
// scan order -- one extra draw is four cards, and no replacement-order ask
// is parked (CR 614.5, 616.1e: the body's own draws still meet the other
// candidate through replDrawChain).
func TestTwoDrawMoresSettleInScanOrder(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 743)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Mr. Spock, Science Officer"))
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Mr. Spock, Science Officer"))
	setDrawHand(t, e, 0, 0)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 0 {
		t.Fatalf("precondition: seat 0 hand = %d, want 0", got)
	}

	before := countDraw(e)
	emitDraw(t, e, 0)
	if got := countDraw(e) - before; got != 4 {
		t.Fatalf("Draw events for one draw beside two draw-mores = %d, want 4 (2 x 2)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != 4 {
		t.Fatalf("seat 0 hand after the competition = %d, want 4", got)
	}
	if d := e.pending; d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("parked a replacement-order ask %+v for a commutative draw-more competition", d)
	}
}
