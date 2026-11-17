package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mondrakBoard is Mondrak beside Katilda (a Human that grants Humans a mana
// ability, untapped and not summoning-sick), a summoning-sick Songstitcher and
// three untapped lands, with the controller at life.
func mondrakBoard(t *testing.T, life int32) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Mondrak, Glory Dominus"))
	mon := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: mon, From: state.ZHand, To: state.ZBattlefield})
	toMain1(t, e)
	for i, n := range []string{"Katilda, Dawnhart Prime", "Songstitcher", "Forest", "Plains", "Barbarian Ring"} {
		o := e.G.AddObject(corpusAlternativeCard(t, n), 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		if i == 0 {
			o.SummonSick = false
		}
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: life - e.G.Players[0].Life})
	e.priorityRound()
	return e, mon
}

// CR 107.4f, 119.4: at life 1 neither {W/P} pip can be paid with life, so the
// potential-play witness for Mondrak's {1}{W/P}{W/P} must fund both pips with
// {W}. The planner's census holds one {W} source (the Plains; Katilda's granted
// mana ability is the grant search's), so no plan pays it. The verdict once
// priced only the {1} and left the pips to life, a witness whose lowering
// strands the play (dirac/oracle lockstep, tail-55 pair 5).
func TestMondrakPotentialWitnessFundsPipsAtLowLife(t *testing.T) {
	t.Parallel()
	verdict := func(life int32) PaymentPlanOutcome {
		e, mon := mondrakBoard(t, life)
		if got := e.G.Players[0].Life; got != life {
			t.Fatalf("precondition: life = %d, want %d", got, life)
		}
		out, _ := e.potentialPlayVerdictShared(0, decision.Option{Kind: "ability", Obj: mon, Ability: 0}, nil)
		return out
	}
	if out := verdict(14); out.Plan == nil || out.Reason != "" || len(out.Plan.Activations) != 1 {
		t.Fatalf("precondition: at life 14 the {1} alone is the witness (pips paid with life): %+v", out)
	}
	if out := verdict(1); out.Plan != nil {
		t.Fatalf("life 1 priced only the mana half, a witness that cannot make the play: %+v", out.Plan)
	}
}

// CR 119.4: a player below 0 life (Platinum Angel keeps them in the game)
// cannot pay even one pip with life, so the potential-play witness clamps to
// k = 0 exactly as the offer walk does (autoPayPhyrexian). The unclamped
// quotient pins the clamp at life -1 only by accident (Go's truncating
// division of -1/2 is 0 already); at life -3 it is -1, below the loop's
// floor, so without the clamp the stranded as-compiled witness of the
// unclamped cost returns — a witness that leaves the pips to life the player
// does not have.
func TestMondrakPotentialWitnessClampsNegativeLife(t *testing.T) {
	t.Parallel()
	for _, life := range []int32{-1, -3} {
		e, mon := mondrakBoard(t, 1)
		e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: life - 1})
		if got := e.G.Players[0].Life; got != life {
			t.Fatalf("precondition: life = %d, want %d", got, life)
		}
		out, _ := e.potentialPlayVerdictShared(0, decision.Option{Kind: "ability", Obj: mon, Ability: 0}, nil)
		if out.Plan != nil {
			t.Fatalf("life %d priced only the mana half, a witness that leaves the pips to life below zero: %+v", life, out.Plan)
		}
	}
}
