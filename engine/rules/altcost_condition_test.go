package rules

// The Force cycle's Condition$ NotPlayerTurn alternative cost (CR 118.9
// scoped by the printed "If it's not your turn": Force of Negation, Force of
// Vigor, Force of Despair; Force of Rage and Force of Virtue the same cycle),
// and Mine Collapse's PlayerTurn twin ("If it's your turn, you may sacrifice
// a Mountain"). Every card is the REAL compiled corpus card loaded by name
// from the gitignored registry -- no Forge script text is copied into this
// file.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// condAltOption returns the alternative-cost cast option (AltCostIndex > 0)
// for obj, or nil.
func condAltOption(opts []decision.Option, obj state.ObjID) *decision.Option {
	for i := range opts {
		o := opts[i]
		if o.Kind == "cast" && o.Obj == obj && o.AltCostIndex > 0 {
			return &o
		}
	}
	return nil
}

// condHandEngine seeds seat 0's hand with exactly the named corpus cards at
// its main phase (handEngine's shape).
func condHandEngine(t *testing.T, names ...string) (*Engine, state.ObjID) {
	t.Helper()
	hand := make([]*cards.Card, 0, len(names))
	for _, n := range names {
		hand = append(hand, corpusAlternativeCard(t, n))
	}
	e := handEngine(t, hand...)
	first, ok := e.G.Zone(state.ZHand, 0)[0], len(e.G.Zone(state.ZHand, 0)) > 0
	if !ok {
		t.Fatalf("precondition: empty hand")
	}
	return e, first
}

// forceOfDespairInHand returns seat 0's hand object of the named card.
func forceOfDespairInHand(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("precondition: no %s in seat 0's hand", name)
	return 0
}

// TestForceCycleAltCostTurnGate: on the caster's own turn the free-cast
// alternative is withheld; on an opponent's turn it is offered and paying it
// exiles the chosen fodder while the spell still resolves (Force of Despair,
// whose DestroyAll asks no target).
func TestForceCycleAltCostTurnGate(t *testing.T) {
	t.Parallel()
	e, _ := condHandEngine(t, "Force of Despair", "Dark Ritual")
	id := forceOfDespairInHand(t, e, "Force of Despair")
	// It is P1's own turn: no alternative-cost cast option; the printed cast
	// is unaffordable ({1}{B}{B} over an empty pool of Mountains).
	if o := condAltOption(e.legalActions(0), id); o != nil {
		t.Fatalf("own turn: alternative-cost cast offered: %+v", o)
	}
	// Not the caster's turn: the alternative is offered.
	e.G.Active = 1
	alt := condAltOption(e.legalActions(0), id)
	if alt == nil {
		t.Fatalf("opponent's turn: no alternative-cost cast option: %+v", e.legalActions(0))
	}
	e.beginCast(0, *alt)
	// The cast asks the fodder exile pick (CR 601.2b): Dark Ritual.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 {
		t.Fatalf("expected the fodder pick, got %+v", d)
	}
	if o := e.G.Obj(d.Options[0].Obj); o == nil || o.Face() == nil || o.Face().Name != "Dark Ritual" {
		t.Fatalf("pick options = %+v, want only Dark Ritual", d.Options)
	}
	submitChoices(t, e, d.Options[0].Index)
	// The pick was paid: the fodder left the hand for exile and the spell
	// sits on the stack to resolve.
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
		t.Fatalf("Force of Despair not on the stack after the pick: %+v", o)
	}
	for _, oid := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Dark Ritual" {
			t.Errorf("Dark Ritual still in hand: the exile pick was not paid")
		}
	}
	if n := len(e.G.Zone(state.ZExile, 0)); n != 1 {
		t.Errorf("exile holds %d cards, want 1 (the paid fodder)", n)
	}
}

// TestMineCollapsePlayerTurnGate: the PlayerTurn twin -- the sacrifice
// alternative is offered on the caster's own turn and withheld off it.
func TestMineCollapsePlayerTurnGate(t *testing.T) {
	t.Parallel()
	e, _ := condHandEngine(t, "Mine Collapse")
	// The Mountain the sacrifice alternative reads, and the creature the
	// spell's DealDamage target walk needs.
	putBattlefield(t, e, 0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	putBattlefield(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mc := forceOfDespairInHand(t, e, "Mine Collapse")
	if o := condAltOption(e.legalActions(0), mc); o == nil {
		t.Fatalf("own turn: no sacrifice alternative offered (precondition): %+v", e.legalActions(0))
	}
	e.G.Active = 1
	if o := condAltOption(e.legalActions(0), mc); o != nil {
		t.Fatalf("opponent's turn: sacrifice alternative offered")
	}
}
