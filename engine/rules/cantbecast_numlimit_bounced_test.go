package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCantBeCastNumLimitBouncedSpellStillCounts: a card cast this turn that
// returned to hand keeps its ObjID, but its finished cast is still a spell
// its caster cast this turn. Under High Noon (NumLimitEachTurn$ 1) the same
// card is not castable again: only an in-flight cast's own push is excluded.
func TestCantBeCastNumLimitBouncedSpellStillCounts(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:A\nManaCost:0\nTypes:Artifact\nOracle:x\n"))
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) != 1 {
		t.Fatalf("hand = %d cards, want 1", len(hand))
	}
	a := hand[0]
	onBoardCard(t, e, 0, corpusCard(t, "High Noon"))
	numLimitCastAndResolve(t, e, a)
	if n := e.SpellsCastThisTurnBy(0); n != 1 {
		t.Fatalf("precondition: seat 0 casts this turn = %d, want 1", n)
	}
	if o := e.G.Obj(a); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: A did not resolve onto the battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: a, From: state.ZBattlefield, To: state.ZHand})
	if o := e.G.Obj(a); o == nil || o.Zone != state.ZHand {
		t.Fatal("precondition: A is not back in hand")
	}
	if !e.castRestricted(0, a) {
		t.Fatal("High Noon lets seat 0 recast its bounced spell: the earlier cast of the same ObjID was excluded")
	}
	numLimitAtPriority(t, e)
	if castOptionNamed(e, a) != nil {
		t.Fatal("bounced spell offered again under High Noon")
	}
}
