package rules

// CR 702.66a: delve reduces the cost of the SPELL with delve on the stack. A
// permanent with Delve on the battlefield (Gurmag Angler carrying a Rock's
// "{1}, {T}, Sacrifice Rock: ..." grant, or any printed activated ability)
// pays an activation's full cost: the offer gate used to credit the owner's
// graveyard against it, offering an activation no mana could fund.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const delvePermanentSrc = "Name:Delve Pump\nManaCost:5 B\nTypes:Creature Fish\nPT:5/5\nK:Delve\n" +
	"A:AB$ Pump | Cost$ 1 | NumAtt$ +1 | SpellDescription$ pump\nOracle:x\n"

func TestDelvePermanentActivationPaysFullCost(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, delvePermanentSrc), card(t, "Name:Husk\nManaCost:G\nTypes:Creature Husk\nPT:1/1\nOracle:x\n"))
	hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	e.emit(events.Event{Kind: events.MoveZone, Obj: hand[0], From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: hand[1], From: state.ZHand, To: state.ZGraveyard})
	if gy := len(e.G.Zone(state.ZGraveyard, 0)); gy == 0 {
		t.Fatalf("precondition: graveyard empty, the Delve credit has nothing to read")
	}
	addMana(t, e, 0, "") // re-asks priority so the pending options read the board built above
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool %v, want empty", e.G.Players[0].Pool)
	}
	if opt := offeredGraveyardAbility(e, hand[0]); opt >= 0 {
		t.Fatalf("CR 702.66a: the {1} activation was offered with an empty pool, paid by Delve: %+v", e.Pending().Options)
	}
	addMana(t, e, 0, "B")
	if opt := offeredGraveyardAbility(e, hand[0]); opt < 0 {
		t.Fatalf("control: the {1} activation is not offered with {B} floating: %+v", e.Pending().Options)
	}
}
