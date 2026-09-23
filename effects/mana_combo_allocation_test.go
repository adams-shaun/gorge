package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestComboAnyResolutionAllocationConsumesOneAmountPerSelection pins the
// direct effMana resume path. A split U/R allocation must emit one of each,
// not Amount copies of each selected symbol.
func TestComboAnyResolutionAllocationConsumesOneAmountPerSelection(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Archive Key\nTypes:Artifact\nA:AB$ Mana | Produced$ Combo Any | Amount$ 2\nOracle:x\n")
	source := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: source.ID, From: state.ZLibrary, To: state.ZBattlefield})
	if source.Zone != state.ZBattlefield {
		t.Fatalf("source precondition zone=%s, want battlefield", source.Zone)
	}

	Resolve(h, &Ctx{Source: source.ID, Controller: 0, ManaChoices: []string{"U", "R"}},
		sa(t, "AB$ Mana | Produced$ Combo Any | Amount$ 2"))
	var blue, red int32
	for _, event := range h.log {
		if event.Kind != events.ManaAdd {
			continue
		}
		switch event.Counter {
		case "U":
			blue += event.Amount
		case "R":
			red += event.Amount
		}
	}
	if blue != 1 || red != 1 {
		t.Fatalf("Combo Any split output U=%d R=%d, want one each", blue, red)
	}
}
