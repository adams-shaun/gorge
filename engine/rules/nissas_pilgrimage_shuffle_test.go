package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nissa's Pilgrimage's main search says Shuffle$ False; the only "Then
// shuffle" in the chain is owed at the Defined$ placement leg. A Defined$
// fetch is otherwise not a search and shuffles nothing, so this pins that the
// deferred search shuffle is still paid exactly once.
func TestNissasPilgrimageSpellMasteryStillShuffles(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Nissa's Pilgrimage", "Giant Growth", "Giant Growth")
	searchMoveByName(t, e, "Giant Growth", state.ZGraveyard)
	searchMoveByName(t, e, "Giant Growth", state.ZGraveyard)
	start := len(e.L.Events)

	_, d := castSearchSpell(t, e, "Nissa's Pilgrimage")
	if d.Kind != decision.KChoose || d.Max != 3 || len(d.Options) < 2 {
		t.Fatalf("precondition: spell-mastery main search = %v max %d with %d options, want KChoose max 3", d.Kind, d.Max, len(d.Options))
	}
	picked := []int{d.Options[0].Index, d.Options[1].Index}
	submitChoices(t, e, picked...)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("precondition: battlefield fetch = %+v, want 2 remembered options", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	shuffles := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("Nissa's Pilgrimage cast emitted %d Shuffle events for seat 0, want exactly 1", shuffles)
	}
}
