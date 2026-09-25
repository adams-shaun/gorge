package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestConditionDefinedChosenCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Eladamri, Korvecdal")
	if !ok {
		t.Fatal("corpus has no Eladamri, Korvecdal")
	}
	gate := cards.ResolveSVar(card.Faces[0].SVars, "DBChangeZone")
	if gate == nil {
		t.Fatal("Eladamri has no compiled DBChangeZone SVar")
	}
	if gate.Params["ConditionDefined"] != "ChosenCard" || strings.TrimSpace(gate.Params["ConditionPresent"]) != "Creature" {
		t.Fatalf("precondition: DBChangeZone gate group/filter = %q/%q, want ChosenCard/Creature", gate.Params["ConditionDefined"], gate.Params["ConditionPresent"])
	}
	h := newHost(t, 2)
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	bear := corpusObject(t, reg, h.g, "Grizzly Bears")
	forest := corpusObject(t, reg, h.g, "Forest")
	if source.Zone != state.ZBattlefield || bear.Zone != state.ZBattlefield || forest.Zone != state.ZBattlefield {
		t.Fatal("precondition: source and compared corpus objects must be on the battlefield")
	}
	if !bear.EffectiveIsCreature() || forest.EffectiveIsCreature() {
		t.Fatalf("precondition: compared filters must differ: bear creature=%v, Forest creature=%v", bear.EffectiveIsCreature(), forest.EffectiveIsCreature())
	}

	ctx := &Ctx{Source: source.ID, Controller: 0, Chosen: []state.Target{{Obj: bear.ID}}, ChosenValid: true}
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("chosen creature: met=%v resolved=%v, want true true", met, resolved)
	}
	ctx.Chosen = []state.Target{{Obj: forest.ID}}
	if met, resolved := conditionMet(h, ctx, gate); met || !resolved {
		t.Fatalf("chosen noncreature: met=%v resolved=%v, want false true", met, resolved)
	}

	// No in-flight selection and no event-backed source selection is an
	// unavailable binding, not a known-empty group: preserve fail-open.
	unbound := &Ctx{Source: source.ID, Controller: 0}
	if _, resolved := conditionMet(h, unbound, gate); resolved {
		t.Fatal("precondition: gate resolved without a chosen-card binding; want unresolved/fail-open")
	}
}
