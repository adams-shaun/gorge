package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// mana_paid_exiled_test.go pins the mana-ability shape of Forge's paid-cost
// list: a Produced$ Any / Amount$ X ability whose X reads
// Exiled$CardManaCost. The exile happens in commitManaDiscard (the mana-cost
// continuation), so the card ids must ride the colour ask and the parked
// continuation into the resolution context; without that, CardManaCost reads
// the empty paid list and Food Chain adds its Plus.1 alone.
//
// The real corpus card is driven through the real activation and colour ask:
// Food Chain's X is "1 plus the exiled creature's mana value", so an MV-3
// creature must add 4 and an MV-1 creature 2 -- both distinct from the
// broken constant 1.

// foodChainActivation finds the pending priority decision's activate option
// for Food Chain, activates it, answers the exile cost with creature and the
// Produced$ Any colour ask with colour. It returns the pool amount added
// (the pool after minus before) so the caller can compare activations.
func foodChainActivation(t *testing.T, e *Engine, chain, creature state.ObjID, colour string) int32 {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("no pending decision before Food Chain activation")
	}
	activate := -1
	for _, opt := range d.Options {
		if opt.Kind == "activate" && opt.Obj == chain {
			activate = opt.Index
		}
	}
	if activate < 0 {
		t.Fatalf("Food Chain mana ability not offered: %+v", d.Options)
	}
	before := e.G.Players[0].Pool
	submitChoices(t, e, activate)
	answerPaidCostAsk(t, e, "mana_exile", creature)
	if o := e.G.Obj(creature); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: paid exile creature zone = %+v, want exile", o)
	}
	// The Produced$ Any ask must be posed at the chosen colour, not skipped:
	// an absent ask would mean the exile list never reached the resolution.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no colour ask after paying Food Chain's cost: %+v", d)
	}
	pick := -1
	for _, opt := range d.Options {
		if opt.Kind == "mana" && opt.ManaSymbol == colour {
			pick = opt.Index
		}
	}
	if pick < 0 {
		t.Fatalf("colour ask offers no %s mana: %+v", colour, d.Options)
	}
	submitChoices(t, e, pick)
	return e.G.Players[0].Pool[colourIndex(colour)] - before[colourIndex(colour)]
}

// colourIndex maps the one-letter Produced$ symbol to the pool slot; the
// ask's ManaSymbol is already a single letter, so this is the identity for
// WUBRG and keeps the test's assertions readable.
func colourIndex(colour string) int {
	switch colour {
	case "G":
		return state.MG
	case "R":
		return state.MR
	case "U":
		return state.MU
	case "B":
		return state.MB
	default:
		return state.MW
	}
}

// TestFoodChainAddsOnePlusExiledManaValue activates the real Food Chain twice,
// exiling an MV-3 and then an MV-1 creature, and asserts 4 then 2 mana -- the
// value follows the exiled card, and both differ from the 1 the unbound
// Ctx.Paid.Exiled produced.
func TestFoodChainAddsOnePlusExiledManaValue(t *testing.T) {
	t.Parallel()
	e, cfg := paidCostEngine(t, []string{"Food Chain", "Eternal Scourge", "Llanowar Elves"}, nil)
	chain := paidCostMoveTo(t, e, 0, "Food Chain", state.ZBattlefield)
	big := paidCostMoveTo(t, e, 0, "Eternal Scourge", state.ZBattlefield)
	small := paidCostMoveTo(t, e, 0, "Llanowar Elves", state.ZBattlefield)
	// Preconditions: the two fuels really have different, nonzero mana
	// values, so the resulting amounts (4 and 2) both differ from the
	// broken constant 1 and from each other.
	if got := e.G.Obj(big).Face().ManaValue(); got != 3 {
		t.Fatalf("precondition: Eternal Scourge MV = %d, want 3", got)
	}
	if got := e.G.Obj(small).Face().ManaValue(); got != 1 {
		t.Fatalf("precondition: Llanowar Elves MV = %d, want 1", got)
	}
	if e.G.Obj(big).Face().ManaValue() == e.G.Obj(small).Face().ManaValue() {
		t.Fatal("precondition: the two exiled creatures must have different mana values")
	}

	if got := foodChainActivation(t, e, chain, big, "G"); got != 4 {
		t.Fatalf("first Food Chain activation added %d green mana, want 4 (1 + MV 3)", got)
	}
	if got := foodChainActivation(t, e, chain, small, "G"); got != 2 {
		t.Fatalf("second Food Chain activation added %d green mana, want 2 (1 + MV 1)", got)
	}
	if got := e.G.Players[0].Pool[state.MG]; got != 6 {
		t.Fatalf("green pool after 4 + 2 = %d, want 6", got)
	}
	replayCheck(t, e, cfg)
}
