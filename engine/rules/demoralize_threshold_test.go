package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDemoralizeThresholdRiderGrantsCantBlock resolves the real Demoralize
// and checks its Threshold half (a bare Condition$ Threshold sub) runs with
// seven graveyard cards and stays skipped with six, with no unmodelled-
// condition Note either way.
func TestDemoralizeThresholdRiderGrantsCantBlock(t *testing.T) {
	for _, n := range []int{6, 7} {
		e := mayPlayBase(t)
		dem := corpusCard(t, "Demoralize")
		src := e.G.AddObject(dem, 0)
		fillGraveyardByEvents(t, e, 0, n)
		blocker := onBoardReady(t, e, 1, metalcraftAttacker)
		if got := e.G.CardCount(state.ZGraveyard, 0); got != n {
			t.Fatalf("n=%d: precondition: graveyard holds %d cards", n, got)
		}
		effects.Resolve(e, &effects.Ctx{Source: src.ID, Controller: 0, SVars: dem.Faces[0].SVars}, dem.Faces[0].Abilities[0])
		_, found := cantBlockKeywordLine(t, e, blocker)
		if found != (n >= 7) {
			t.Errorf("n=%d: can't-block keyword found=%v, want %v", n, found, n >= 7)
		}
		if got := unmodelledNotes(e); got != 0 {
			t.Errorf("n=%d: %d unmodelled-condition Notes, want 0", n, got)
		}
	}
}
