package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestOathOfDruidsTargetsTheOpponentWithMoreCreatures: Oath of Druids'
// upkeep trigger names `Player.OpponentToActive+withMoreCreaturesThanActive`
// -- an opponent of the ACTIVE player who controls strictly more creatures
// than the active player. The Oath belongs to seat 1 so a spec read against
// the Oath's controller (Player.Opponent) would offer seat 0 instead.
func TestOathOfDruidsTargetsTheOpponentWithMoreCreatures(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	if e.G.Active != 0 {
		t.Fatalf("precondition: active seat is %d, want 0", e.G.Active)
	}
	oath := onBoardCard(t, e, 1, corpusCard(t, "Oath of Druids"))
	face := e.G.Obj(oath).Face()
	if len(face.Triggers) != 1 || face.Triggers[0].Effect == nil {
		t.Fatalf("Oath of Druids carries %d triggers", len(face.Triggers))
	}
	ab := face.Triggers[0].Effect
	seat0, seat1 := state.Target{Player: 0, IsPlayer: true}, state.Target{Player: 1, IsPlayer: true}

	// Tied at zero creatures: nobody controls MORE than the active player.
	if got := e.LegalTargets(1, oath, ab); len(got) != 0 {
		t.Fatalf("tied creature counts offer %+v", got)
	}
	// The opponent (seat 1) controls a creature the active player lacks.
	onBoardCard(t, e, 1, corpusCard(t, "Grizzly Bears"))
	got := e.LegalTargets(1, oath, ab)
	if !slices.Contains(got, seat1) || slices.Contains(got, seat0) {
		t.Fatalf("seat 1 leads in creatures; targets = %+v", got)
	}
	// The active player catches up: tie again, no target.
	onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	if got := e.LegalTargets(1, oath, ab); len(got) != 0 {
		t.Fatalf("tied again but offers %+v", got)
	}
	// The active player leads: the opponent controls fewer, still no target,
	// and the active player is never its own opponent.
	onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	if got := e.LegalTargets(1, oath, ab); len(got) != 0 {
		t.Fatalf("active player leads but offers %+v", got)
	}
}
