package rules

// CR 605.1a: an activated ability whose cost or effect moves a card to or
// from a library is not a mana ability. Chromatic Sphere ("{1}, {T},
// Sacrifice Chromatic Sphere: Add one mana of any color. Draw a card.") and
// Millikin ("{T}, Mill a card: Add {C}.") are offered as ordinary activated
// abilities, wait on the stack, and add their mana (and draw) on resolution.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestCR605_1aLibraryMovingManaAbilitiesUseTheStack(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 605, map[string]state.Zone{
		"Chromatic Sphere": state.ZBattlefield, "Millikin": state.ZBattlefield})
	sphere, millikin := ids["Chromatic Sphere"], ids["Millikin"]
	e.G.Obj(millikin).SummonSick = false
	for _, id := range []state.ObjID{sphere, millikin} {
		f := e.G.Obj(id).Face()
		if hasOptionKindFor(e, "activate", id) {
			t.Fatalf("CR 605.1a %s seq %d: offered as a mana-ability activation; its cost or effect moves a library card", f.Name, len(e.L.Events))
		}
		if ab := f.Abilities[0]; ab.API != "Mana" {
			t.Fatalf("precondition: %s ability 0 is not an AB$ Mana: %s", f.Name, ab.Line)
		}
	}

	// Millikin: the mill is paid at activation, the {C} arrives on resolution.
	lib := len(e.G.Zone(state.ZLibrary, 0))
	submitChoices(t, e, abilityOption(t, e, millikin, 0).Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("CR 605.1a Millikin seq %d: stack depth %d after activation, want the ability on the stack", len(e.L.Events), len(e.G.Stack))
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib-1 {
		t.Fatalf("CR 605.1a Millikin seq %d: library %d after paying Mill<1>, want %d", len(e.L.Events), got, lib-1)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("CR 605.1a Millikin seq %d: mana arrived before the ability resolved: %v", len(e.L.Events), e.G.Players[0].Pool)
	}
	passToResolution(t, e)
	if got := e.G.Players[0].Pool[state.MC]; got != 1 {
		t.Fatalf("CR 605.1a Millikin seq %d: pool %v after resolution, want {C}", len(e.L.Events), e.G.Players[0].Pool)
	}

	// Chromatic Sphere: {1} from the floating {C}; the draw and the mana
	// both wait for resolution.
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, abilityOption(t, e, sphere, 0).Index)
	for i := 0; i < 8 && len(e.G.Stack) == 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("CR 605.1a Chromatic Sphere seq %d: stack depth %d after activation, want the ability on the stack", len(e.L.Events), len(e.G.Stack))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
		t.Fatalf("CR 605.1a Chromatic Sphere seq %d: hand %d before resolution, want %d (the draw waits)", len(e.L.Events), got, hand)
	}
	passToResolution(t, e)
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index) // the colour ask
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Errorf("CR 605.1a Chromatic Sphere seq %d: hand %d after resolution, want %d", len(e.L.Events), got, hand+1)
	}
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Errorf("CR 605.1a Chromatic Sphere seq %d: pool %v after resolution, want one mana", len(e.L.Events), e.G.Players[0].Pool)
	}
	replayCheck(t, e, cfg)
}
