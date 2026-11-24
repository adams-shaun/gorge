// oracle_trigger_cost_posed_payable_test.go — the oracle CostPosed flag
// mirrors XMage's DoIfCostPaid: its chooseUse is posed exactly when the
// cost's non-mana components can be paid now (PayLifeCost.canPay: life >= N;
// an exile cost: a candidate exists; CollectEvidenceCost.canPay: the
// graveyard's total mana value reaches N). Mana itself never gates the ask.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	posedMV3Src = "Name:Posed Three\nManaCost:2 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	posedMV2Src = "Name:Posed Two\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// posedToGrave moves seat 0's named card from its hand or library to its
// graveyard through the event fold.
func posedToGrave(t *testing.T, e *Engine, name string) {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
				return
			}
		}
	}
	t.Fatalf("seat 0 holds no %q in hand or library", name)
}

func TestXmagePosesTriggerCostByPayability(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 7, nil, []string{posedMV3Src, posedMV2Src}, nil)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life %d, want 20", got)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 0 {
		t.Fatalf("precondition: graveyard holds %d cards, want 0", got)
	}
	ask := func(label string) bool { return xmagePosesTriggerCost(e, 0, 0, e.parseCost(label)) }
	check := func(stage, label string, want bool) {
		t.Helper()
		if got := ask(label); got != want {
			t.Errorf("%s: posed(%q) = %v, want %v", stage, label, got, want)
		}
	}
	// Always-posed shapes.
	check("empty grave", "1 B", true)
	check("empty grave", "1 B Discard<1/Hand>", true)
	// A sacrifice keeps its old reading: not posed.
	check("empty grave", "1 B Sac<1/Creature>", false)
	// Mana + life: posed while life covers N.
	check("empty grave", "W B PayLife<2>", true)
	check("empty grave", "W B PayLife<25>", false)
	// Mana + exile from the graveyard: needs a candidate.
	check("empty grave", "1 ExileFromGrave<1/Card>", false)
	// Collect evidence 4 over an empty graveyard.
	check("empty grave", "CollectEvidence<4>", false)

	posedToGrave(t, e, "Posed Three")
	check("grave MV 3", "1 ExileFromGrave<1/Card>", true)
	check("grave MV 3", "CollectEvidence<4>", false)
	check("grave MV 3", "1 CollectEvidence<4>", false)

	posedToGrave(t, e, "Posed Two")
	check("grave MV 5", "CollectEvidence<4>", true)
	check("grave MV 5", "1 CollectEvidence<4>", true)
	check("grave MV 5", "W B PayLife<2> CollectEvidence<4>", true)
	check("grave MV 5", "W B PayLife<25> CollectEvidence<4>", false)
}
