package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// spell_ispresent_cast_gate_test.go pins CR 601.3 for a spell's top-level
// IsPresent$: Confront the Assault ("Cast this spell only if a creature is
// attacking you", A:SP$ Token | IsPresent$ Creature.attackingYou) is offered
// only while a creature attacks its caster. spellTimingPre applies
// abilityPresentHolds to the spell's own SA, so every cast route that asks
// spellTimingOK is gated.

// confrontEngine builds seat 0 (Confront the Assault in its deck) vs seat 1
// (Ally Bear), seat 1's bear already on the battlefield so it can attack on
// turn 2, and returns the engine plus Confront's and the bear's ids.
func confrontEngine(t *testing.T) (e *Engine, confront, bear state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	ctat := searchCorpusCard(t, reg, "Confront the Assault")
	e, _ = activationDeck(t, 4412, []*cards.Card{ctat}, []*cards.Card{card(t, actPhaseAllyBearSrc)})
	confront = confrontHandOrMove(t, e, 0, "Confront the Assault")
	bear = confrontHandOrMove(t, e, 1, "Ally Bear")
	confrontToBattlefield(t, e, bear)
	return e, confront, bear
}

// confrontHandOrMove returns the named card's id, moving it to seat p's hand from the
// library when the opening hand did not already hold it.
func confrontHandOrMove(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return moveByName(t, e, p, name, state.ZHand)
}

func confrontToBattlefield(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	if o := e.G.Obj(id); o.Zone == state.ZBattlefield {
		return
	}
	moveByName(t, e, e.G.Obj(id).Controller, e.G.Obj(id).Face().Name, state.ZBattlefield)
}

// confrontPassUntilSeat passes priority until seat p holds a priority decision.
func confrontPassUntilSeat(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	for i := 0; i < 50; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind == decision.KPriority && d.Player == p {
			return
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %s decision: %+v", d.Kind, d)
		}
		submitPass(t, e)
	}
	t.Fatalf("seat %d never received priority", p)
}

func TestSpellIsPresentGatesCast(t *testing.T) {
	t.Parallel()

	t.Run("own main phase", func(t *testing.T) {
		t.Parallel()
		e, confront, bear := confrontEngine(t)
		if o := e.G.Obj(bear); o.IsAttacking {
			t.Fatal("precondition: the bear is attacking on turn 1")
		}
		fundPoolSeat(t, e, 0, "WWWWW")
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("precondition: want seat 0 priority, got %+v", d)
		}
		if castOffered(e, confront) {
			t.Fatal("Confront the Assault offered in seat 0's own main phase with no attacker")
		}
	})

	t.Run("opponent attacks the caster", func(t *testing.T) {
		t.Parallel()
		e, confront, bear := confrontEngine(t)
		driveToStep(t, e, 2, 1, state.StepDeclareAttackers)
		passToKind(t, e, decision.KAttackers)
		submitAttackersOnly(t, e, bear)
		confrontPassUntilSeat(t, e, 0)
		if o := e.G.Obj(bear); !o.IsAttacking || o.Attacking != 0 {
			t.Fatalf("precondition: bear attacking=%v defender=%d, want attacking seat 0", o.IsAttacking, o.Attacking)
		}
		fundPoolSeat(t, e, 0, "WWWWW")
		if !castOffered(e, confront) {
			t.Fatalf("Confront the Assault not offered while a creature attacks seat 0: %+v", e.Pending())
		}
	})

	t.Run("opponent attacks nobody", func(t *testing.T) {
		t.Parallel()
		e, confront, bear := confrontEngine(t)
		driveToStepAll(t, e, 2, 1, state.StepEnd)
		confrontPassUntilSeat(t, e, 0)
		if o := e.G.Obj(bear); o.Zone != state.ZBattlefield || o.IsAttacking {
			t.Fatalf("precondition: bear zone=%v attacking=%v", o.Zone, o.IsAttacking)
		}
		fundPoolSeat(t, e, 0, "WWWWW")
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("precondition: want seat 0 priority, got %+v", d)
		}
		if castOffered(e, confront) {
			t.Fatal("Confront the Assault offered on seat 1's turn with nobody attacking")
		}
	})
}
