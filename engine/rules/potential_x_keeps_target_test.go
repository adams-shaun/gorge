package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// For the Common Good's only target is the Treasure. A witness paying the largest X
// sacrifices it, so the cast is priced at the X the lands pay; with no land its only
// witness eats the target (CR 601.2c, 601.2h: the target is chosen before the payment).
func TestPotentialXCastKeepsItsOnlyTarget(t *testing.T) {
	t.Parallel()
	verdict := func(forests int) (string, int) {
		e := handEngineTokens(t, corpusCard(t, "For the Common Good"))
		e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
		for i := 0; i < forests; i++ {
			addToBattlefield(t, e, corpusCard(t, "Forest"), 0)
		}
		e.pending = nil
		e.priorityRound()
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" {
				n := 0
				if pp.Plan != nil {
					n = len(pp.Plan.Activations)
				}
				return pp.Detail, n
			}
		}
		t.Fatal("the cast is not a potential play")
		return "", 0
	}
	if detail, _ := verdict(0); detail != "witness_unreachable" {
		t.Fatalf("Treasure alone: detail %q, want witness_unreachable", detail)
	}
	if detail, n := verdict(4); detail != "" || n != 3 {
		t.Fatalf("four Forests: detail %q and %d activations, want a witness of the three Forests that pay X=1", detail, n)
	}
}
