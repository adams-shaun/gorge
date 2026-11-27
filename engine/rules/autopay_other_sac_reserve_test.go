package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A spell whose own additional cost sacrifices a creature competes with an
// other-sacrifice mana source for its victims: the plan never names the
// creature the cost needs.

const otherSacFlingSrc = "Name:OtherSac Fling\nManaCost:2\nTypes:Instant\nA:SP$ Draw | Cost$ 2 Sac<1/Creature> | NumCards$ 1\nOracle:x\n"

// otherSacOffered returns the spell's payment action and whether any plan
// is offered for it.
func otherSacOffered(t *testing.T, e *Engine, spell state.ObjID) (decision.PaymentAction, bool) {
	t.Helper()
	d := ppPriority(t, e)
	d.PaymentActions = e.PaymentActionsForPriority(0, d.Seq)
	for _, a := range d.PaymentActions {
		if a.Cast.Object == spell {
			return a, len(a.Plans) > 0
		}
	}
	return decision.PaymentAction{}, false
}

// TestAutopayAltarNeverEatsAdditionalCostVictim: with one creature the altar
// would consume the only creature the additional cost can sacrifice, so no
// plan is offered; with a second creature the plan names a victim and leaves
// the other for the cost, and accepting it pays both.
func TestAutopayAltarNeverEatsAdditionalCostVictim(t *testing.T) {
	t.Parallel()
	t.Run("one creature", func(t *testing.T) {
		t.Parallel()
		e, _, spell := newFixtureDeck(t, 4713, otherSacFlingSrc)
		onBoard(t, e, 0, otherSacAltarSrc)
		creature := onBoard(t, e, 0, otherSacCreatureSrc)
		if e.G.Obj(creature).Zone != state.ZBattlefield {
			t.Fatalf("precondition: creature zone = %s", e.G.Obj(creature).Zone)
		}
		if a, ok := otherSacOffered(t, e, spell); ok {
			t.Fatalf("a plan is offered although the altar would sacrifice the creature the cost needs: %+v", a.Plans)
		}
	})
	t.Run("two creatures", func(t *testing.T) {
		t.Parallel()
		e, _, spell := newFixtureDeck(t, 4714, otherSacFlingSrc)
		altar := onBoard(t, e, 0, otherSacAltarSrc)
		c1 := onBoard(t, e, 0, otherSacCreatureSrc)
		c2 := onBoard(t, e, 0, otherSacCreatureSrc)
		a, ok := otherSacOffered(t, e, spell)
		if !ok {
			t.Fatalf("no plan offered with a creature to spare for the cost")
		}
		act := a.Plans[0].Activations
		if len(act) != 1 || act[0].Source != altar || len(act[0].Sacrifices) != 1 {
			t.Fatalf("witness %+v, want one altar step with one victim", act)
		}
		if v := act[0].Sacrifices[0]; v != c1 && v != c2 {
			t.Fatalf("victim %d is not one of the creatures %d, %d", v, c1, c2)
		}
		submitPayment(t, e, a)
		// The cast began: the spell left the hand, or the additional cost's own
		// sacrifice ask is pending (a creature is left to answer it).
		if d := e.Pending(); e.G.Obj(spell).Zone == state.ZHand && (d == nil || d.Kind != decision.KChoose) {
			t.Fatalf("spell zone = %s, pending %+v, want the cast begun", e.G.Obj(spell).Zone, d)
		}
	})
}
