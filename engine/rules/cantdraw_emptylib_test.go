package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCantDrawEmptyLibraryNoLoss pins CR 121.6 with 704.5b: a player who
// can't draw because of an effect does not attempt the draw, so an empty
// library under a CantDraw static (Mornsong Aria) marks nothing and the
// state-based check does not end the game for them. The control half drops
// the static and shows the same draw does mark and lose, so the pass cannot
// come from the draw path being inert.
func TestCantDrawEmptyLibraryNoLoss(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	aria := mustCorpusCard(t, reg, "Mornsong Aria")

	draws := map[string]func(e *Engine){
		"effects draw":    func(e *Engine) { effects.DrawFor(e, 1) },
		"draw cost":       func(e *Engine) { e.drawCostCard(1) },
		"declined dredge": func(e *Engine) { e.resumeOrdinaryDraw(1) },
	}
	for name, draw := range draws {
		t.Run(name+"/control", func(t *testing.T) {
			e, cfg := cantDrawFixture(t, 9121, aria)
			cantLoseEmptyLibrary(t, e, 1)
			draw(e)
			if !e.G.Players[1].DrewFromEmpty {
				t.Fatal("control: the empty-library draw did not mark seat 1")
			}
			e.checkStateBased()
			if !e.G.Players[1].Lost {
				t.Fatal("control: seat 1 did not lose to the empty-library draw")
			}
			replayCheck(t, e, cfg)
		})
		t.Run(name, func(t *testing.T) {
			e, cfg := cantDrawFixture(t, 9121, aria)
			putCantDrawCarrierOnBattlefield(t, e, aria.Faces[0].Name)
			cantLoseEmptyLibrary(t, e, 1)
			if got := len(e.G.Zone(state.ZLibrary, 1)); got != 0 {
				t.Fatalf("precondition: seat 1 library = %d cards, want 0", got)
			}
			if !e.drawForbidden(1) {
				t.Fatal("precondition: CantDraw does not forbid seat 1's draw")
			}
			draw(e)
			if e.G.Players[1].DrewFromEmpty {
				t.Fatal("seat 1 was marked for drawing from an empty library although it can't draw")
			}
			if got := countDrawEmpty(e, 1, 0); got != 0 {
				t.Fatalf("draw_empty marks = %d, want 0", got)
			}
			e.checkStateBased()
			if e.G.Players[1].Lost || e.G.Over {
				t.Fatalf("seat 1 lost=%v over=%v after a prevented draw from an empty library", e.G.Players[1].Lost, e.G.Over)
			}
			replayCheck(t, e, cfg)
		})
	}
}
