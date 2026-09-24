package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestPresentDefinedTriggeredSourceLKICopy(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:PresentSource\nTypes:Creature\nPT:2/2\nOracle:x\n")
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	if source.Zone != state.ZBattlefield {
		t.Fatal("triggered source fixture is not on the battlefield")
	}
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | Present$ Creature | PresentCompare$ EQ1")
	ctx := &Ctx{Controller: 0, Source: source.ID, TriggerContext: TriggerContext{TriggerSource: source.ID}}
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("matching triggered source: met=%v resolved=%v, want true true", met, resolved)
	}
	// A present compare against a different type has zero matching referents.
	gate = sa(t, "DB$ SetState | PresentDefined$ TriggeredSourceLKICopy | Present$ Land | PresentCompare$ EQ1")
	if met, resolved := conditionMet(h, ctx, gate); met || !resolved {
		t.Fatalf("no matching source referent: met=%v resolved=%v, want false true", met, resolved)
	}
}
