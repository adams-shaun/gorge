// An equip whose only payable price comes from a target-dependent
// ReduceCost static (Dwarven Mauler's "equip abilities that target this
// creature cost {2} less", ValidTarget$ Card.Self) must be offered from
// untapped lands alone: CR 601.2f totals the cost after CR 601.2c's target
// choice, so the autopay witness (planAbilityPayment) has to price the same
// best-target discount the offer gate's potential-target retry prices. The
// re-equip case (the Equipment already on the Mauler, CR 702.6a keeps it a
// legal target) is the shape the std-hob lockstep pair diverged on.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	reattachMaulerSrc = "Name:Mauler\nManaCost:R\nTypes:Creature Dwarf Warrior\nPT:2/1\nS:Mode$ ReduceCost | Amount$ 2 | Activator$ You | ValidSpell$ Activated.Equip | ValidTarget$ Card.Self | Description$ Equipping me is two cheaper.\nOracle:x\n"
	reattachSpearSrc  = "Name:Spear\nManaCost:1 R\nTypes:Artifact Equipment\nS:Mode$ Continuous | Affected$ Creature | AddPower$ 2 | AffectedDefined$ Equipped | Description$ Bearer is stronger.\nK:Equip:3\nOracle:x\n"
	reattachLandSrc   = "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
)

// reattachGame parks seat 0 at turn 3 Main1 with the Mauler, the Spear and
// exactly two untapped Mountains it controls (lands), the pool empty. With
// attached, the Spear is first equipped to the Mauler from a floated {C}.
func reattachGame(t *testing.T, seed uint64, attached bool) (e *Engine, mauler, spear state.ObjID, lands []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{card(t, reattachMaulerSrc), card(t, reattachSpearSrc)}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e = New(seatZeroStart(cfg))
	e.Advance()
	mauler = moveSeeded(t, e, 0, reattachMaulerSrc, state.ZBattlefield)
	spear = moveSeeded(t, e, 0, reattachSpearSrc, state.ZBattlefield)
	lands = []state.ObjID{
		moveSeeded(t, e, 0, reattachLandSrc, state.ZBattlefield),
		moveSeeded(t, e, 0, reattachLandSrc, state.ZBattlefield),
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	if attached {
		addMana(t, e, 0, "C")
		opt, ok := findAbilityOption(e, spear, 0)
		if !ok {
			t.Fatal("first equip not offered from {C} against the Mauler")
		}
		submitChoices(t, e, opt.Index)
		targetObject(t, e, mauler)
		passUntilStackEmpty(t, e, 20)
		if e.G.Obj(spear).AttachedTo != mauler {
			t.Fatal("precondition: the Spear is not attached to the Mauler")
		}
	}
	return e, mauler, spear, lands
}

func untappedCount(e *Engine, ids []state.ObjID) int {
	n := 0
	for _, id := range ids {
		if !e.G.Obj(id).Tapped {
			n++
		}
	}
	return n
}

func TestEquipTargetDiscountOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	for _, attached := range []bool{false, true} {
		e, mauler, spear, lands := reattachGame(t, 611, attached)
		// Preconditions: the undiscounted {3} is NOT payable (empty pool, two
		// lands), so only the Mauler's {2} reduction makes the equip legal.
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("attached=%v: pool = %d, want 0", attached, got)
		}
		if got := untappedCount(e, lands); got != 2 {
			t.Fatalf("attached=%v: untapped Mountains = %d, want 2", attached, got)
		}
		opt, ok := findAbilityOption(e, spear, 0)
		if !ok {
			t.Fatalf("attached=%v: equip {3} with a {2}-less target not offered from two untapped Mountains", attached)
		}
		submitChoices(t, e, opt.Index)
		targetObject(t, e, mauler)
		passUntilStackEmpty(t, e, 20)
		if e.G.Obj(spear).AttachedTo != mauler {
			t.Fatalf("attached=%v: Spear attached to %d, want the Mauler %d", attached, e.G.Obj(spear).AttachedTo, mauler)
		}
		// Exactly {1} was charged: one Mountain tapped, one left.
		if got := untappedCount(e, lands); got != 1 {
			t.Fatalf("attached=%v: untapped Mountains after equipping = %d, want 1 ({3} - {2} = {1} charged)", attached, got)
		}
	}
}
