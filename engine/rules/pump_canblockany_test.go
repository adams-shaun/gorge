// CR 509.1a / 611.2a. A resolved Pump's CanBlockAny$ True ("target creature
// can block any number of creatures this turn": Give No Ground, Valor Made
// Real) lets its target be declared against several attackers for the rest of
// the turn, and not the next. Authored fixtures: no corpus card is loaded.
package rules

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	canBlockAnySpellSrc = "Name:Valor Stand-In\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ Pump | CanBlockAny$ True | ValidTgts$ Creature | SpellDescription$ Wide guard.\nOracle:x\n"
)

func TestCR509_1aPumpCanBlockAnyLastsThisTurn(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	atks := []state.ObjID{onBoardReady(t, e, 0, atk22Src), onBoardReady(t, e, 0, atk22Src)}
	blk := onBoard(t, e, 1, plainSevenSrc)
	so := e.G.AddObject(card(t, canBlockAnySpellSrc), 1)
	so.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), so.ID))
	spell := so.ID
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "W", Amount: 1})
	if got := combat.BlockCapacity(asBoard(e), blk); got != 1 {
		t.Fatalf("precondition: blocker capacity before the spell = %d, want 1", got)
	}

	e.askAttackers()
	submitAttackersOnly(t, e, atks...)
	// Seat 0 passes the declare-attackers window; seat 1 then holds priority.
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player == 1 {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	castSpellAtTarget(t, e, spell, blk)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("no blockers decision: %+v", d)
	}
	group, n := blockerOptions(d, blk)
	if n != 2 || group == "" {
		t.Fatalf("precondition: blocker has %d options in group %q, want 2", n, group)
	}
	if got := combat.BlockCapacity(asBoard(e), blk); got != math.MaxInt32 {
		t.Fatalf("BlockCapacity this turn = %d, want unbounded", got)
	}
	if got := d.GroupLimits[group]; got != 2 {
		t.Fatalf("GroupLimits[%q] = %d, want 2 (%v)", group, got, d.GroupLimits)
	}
	submitBlockerPairsOnly(t, e, [2]state.ObjID{atks[0], blk}, [2]state.ObjID{atks[1], blk})
	for _, a := range atks {
		if bb := e.G.Obj(a).BlockedBy; len(bb) != 1 || bb[0] != blk {
			t.Fatalf("attacker %d BlockedBy = %v, want [%d]", a, bb, blk)
		}
	}

	// The grant ends with the turn (CR 514.2): the next turn's capacity is 1.
	e.EndOfTurnCleanup()
	if got := combat.BlockCapacity(asBoard(e), blk); got != 1 {
		t.Fatalf("BlockCapacity next turn = %d, want 1", got)
	}
}
