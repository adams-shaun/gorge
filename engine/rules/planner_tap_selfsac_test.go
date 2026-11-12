package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tomb Tyrant's shape: a "{2}{B}, {T}, Sacrifice a creature" ability whose
// source is the only creature its sacrifice part could take. CR 602.2b pays
// the costs in any order, so the source is tapped by the cost's {T} and then
// sacrificed: the plan must still find the cost payable and the option must
// be offered (the planner's sacrifice census used to spend id on the tap and
// leave the Sac part with no candidate, cost:reserved_sources).
const tapSelfSacSrc = "Name:Tyrant\nManaCost:3 B\nTypes:Creature Zombie Noble\nPT:3/3\n" +
	"A:AB$ Draw | Cost$ 2 B T Sac<1/Creature/creature> | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\n" +
	"Oracle:x\n"

const swampSrc = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"

// tapSacOption returns the pending priority decision's "ability" option for
// id, or nil.
func tapSacOption(e *Engine, id state.ObjID) *decision.Option {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind == "ability" && o.Obj == id {
			return o
		}
	}
	return nil
}

// TestTapAndSelfSacrificeCostIsPlanned: the source may pay both the {T} and
// the Sac<1/Creature> of its own ability (CR 602.2b). The mana comes from
// untapped lands (the game's usual shape): the floating pool would price the
// offer before the planner's reservation walk is ever reached.
func TestTapAndSelfSacrificeCostIsPlanned(t *testing.T) {
	t.Parallel()
	e, _, tyrant := newFixtureDeck(t, 91, tapSelfSacSrc, swampSrc, swampSrc, swampSrc)
	tyrant = putCreature(t, e, 0, tapSelfSacSrc)
	e.G.Obj(tyrant).SummonSick = false
	putCreature(t, e, 0, swampSrc)
	putCreature(t, e, 0, swampSrc)
	putCreature(t, e, 0, swampSrc)
	addMana(t, e, 0, "") // re-ask priority so the offer reflects the moved permanents
	if tapSacOption(e, tyrant) == nil {
		t.Fatalf("tap+self-sacrifice cost not offered with the source as the only creature: %+v", e.Pending())
	}

	// The source is actually tapped and then sacrificed.
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected priority, got %+v", e.Pending())
	}
	o := tapSacOption(e, tyrant)
	if o == nil {
		t.Fatal("ability option gone before activation")
	}
	submitChoices(t, e, o.Index)
	// CR 601.2h: the Sac part is picked at payment.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" {
		t.Fatalf("sacrifice decision %+v", d)
	}
	submitChoices(t, e, 0)
	ab := e.G.Obj(tyrant)
	if ab.Zone != state.ZGraveyard {
		t.Fatalf("the sacrificed source is in %v, want the graveyard", ab.Zone)
	}
	if len(e.G.Stack) == 0 {
		t.Fatalf("the ability is not on the stack")
	}

	// A tapped source cannot pay the {T}: no offer.
	e2, _, tyrant2 := newFixtureDeck(t, 92, tapSelfSacSrc, swampSrc, swampSrc, swampSrc)
	tyrant2 = putCreature(t, e2, 0, tapSelfSacSrc)
	e2.G.Obj(tyrant2).SummonSick = false
	putCreature(t, e2, 0, swampSrc)
	putCreature(t, e2, 0, swampSrc)
	putCreature(t, e2, 0, swampSrc)
	e2.G.Obj(tyrant2).Tapped = true
	for _, o := range e2.legalActions(0) {
		if o.Kind == "ability" && o.Obj == tyrant2 {
			t.Fatalf("offered while the source is tapped: %+v", o)
		}
	}
}
