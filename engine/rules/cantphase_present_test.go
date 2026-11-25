package rules

// CR 702.26 -- The Pandorica's Effect body registers
// `Mode$ CantPhaseIn | ValidCard$ Card.phasedOutIsRemembered | IsPresent$
// Card.EffectSource+tapped`: the remembered permanent can't phase in for as
// long as The Pandorica remains tapped, and phases in (TrigUntaps) once it
// untaps.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// pandoricaPhasedOut activates The Pandorica on p1's Memnite and returns the
// engine with Memnite phased out and The Pandorica tapped.
func pandoricaPhasedOut(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg := tokenReplGameSeats(t, 1601,
		[]*cards.Card{corpusAlternativeCard(t, "The Pandorica")},
		[]*cards.Card{cr702corpusCard(t, "Memnite")})
	memnite := moveSeededCard(t, e, 1, cr702corpusCard(t, "Memnite"), state.ZBattlefield)
	toMain1(t, e)
	pand := moveByName(t, e, 0, "The Pandorica", state.ZBattlefield)
	addMana(t, e, 0, "1W")
	submitChoices(t, e, abilityOption(t, e, pand, 0).Index)
	cantPhaseTarget(t, e, memnite)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(pand); o == nil || !o.Tapped {
		t.Fatal("precondition: The Pandorica is not tapped after its activation")
	}
	if o := e.G.Obj(memnite); o == nil || !o.PhasedOut {
		t.Fatal("precondition: the ability did not phase Memnite out")
	}
	if !cantPhaseRegistered(t, e, effects.ModeCantPhaseIn) {
		t.Fatal("precondition: no CantPhaseIn restriction registered by the Effect")
	}
	if n := pandoricaRegistrations(e, pand); n != 2 {
		t.Fatalf("precondition: want the Untaps and ChangesZone comeback registrations, got %d", n)
	}
	return e, cfg, pand, memnite
}

// pandoricaRegistrations counts The Pandorica's live comeback registrations.
func pandoricaRegistrations(e *Engine, pand state.ObjID) int {
	n := 0
	for _, dt := range e.G.Delayed {
		if dt.Source == pand {
			n++
		}
	}
	return n
}

// pandoricaComeback runs the held-then-untapped half and returns with Memnite
// phased back in, in p0's first main phase.
func pandoricaComeback(t *testing.T, e *Engine, pand, memnite state.ObjID) {
	t.Helper()
	driveToStepAll(t, e, e.G.Turn+1, 1, state.StepUpkeep)
	driveToStepAll(t, e, e.G.Turn+1, 0, state.StepUpkeep)
	driveToStepAll(t, e, e.G.Turn, 0, state.StepMain1)
	if o := e.G.Obj(memnite); o == nil || o.PhasedOut {
		t.Fatalf("precondition: Memnite did not phase in once The Pandorica untapped: %+v", o)
	}
	if n := pandoricaRegistrations(e, pand); n != 0 {
		t.Fatalf("ForgetOnPhasedIn$: %d comeback registrations outlive the phase-in", n)
	}
}

// The comeback body is a toggle, so a registration that outlived the phase-in
// would phase the returned permanent OUT on The Pandorica's next untap.
func TestPandoricaComebackDoesNotRepeatOnLaterUntap(t *testing.T) {
	t.Parallel()
	e, cfg, pand, memnite := pandoricaPhasedOut(t)
	pandoricaComeback(t, e, pand, memnite)
	e.emit(events.Event{Kind: events.Tap, Obj: pand})
	driveToStepAll(t, e, e.G.Turn+1, 1, state.StepUpkeep)
	driveToStepAll(t, e, e.G.Turn+1, 0, state.StepUpkeep)
	driveToStepAll(t, e, e.G.Turn, 0, state.StepMain1)
	if o := e.G.Obj(pand); o == nil || o.Tapped {
		t.Fatal("precondition: The Pandorica did not untap a second time")
	}
	if o := e.G.Obj(memnite); o == nil || o.PhasedOut {
		t.Fatalf("Memnite phased out again when The Pandorica untapped a second time: %+v", o)
	}
	replayCheck(t, e, cfg)
}

// The same promise on the leave-the-battlefield half.
func TestPandoricaComebackDoesNotRepeatWhenItLeaves(t *testing.T) {
	t.Parallel()
	e, cfg, pand, memnite := pandoricaPhasedOut(t)
	pandoricaComeback(t, e, pand, memnite)
	moveToGraveyard(t, e, pand)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(pand); o == nil || o.Zone == state.ZBattlefield {
		t.Fatal("precondition: The Pandorica did not leave the battlefield")
	}
	if o := e.G.Obj(memnite); o == nil || o.PhasedOut {
		t.Fatalf("Memnite phased out when The Pandorica left after the comeback: %+v", o)
	}
	replayCheck(t, e, cfg)
}

func TestPandoricaHoldsPhasedOutWhileTapped(t *testing.T) {
	t.Parallel()
	e, cfg, pand, memnite := pandoricaPhasedOut(t)
	// p1's untap step with The Pandorica still tapped: Memnite stays out.
	driveToStepAll(t, e, e.G.Turn+1, 1, state.StepUpkeep)
	if o := e.G.Obj(memnite); o == nil || !o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("CR 702.26: Memnite phased in while The Pandorica remains tapped: %+v", o)
	}
	if n := cantPhaseEvents(e, memnite, false); n != 0 {
		t.Fatalf("%d phase-in events while The Pandorica remains tapped", n)
	}

	// p0's untap step: the election untaps The Pandorica (first option), its
	// "becomes untapped" trigger phases Memnite in.
	driveToStepAll(t, e, e.G.Turn+1, 0, state.StepUpkeep)
	if o := e.G.Obj(pand); o == nil || o.Tapped {
		t.Fatal("precondition: The Pandorica did not untap at its controller's untap step")
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepMain1)
	if o := e.G.Obj(memnite); o == nil || o.PhasedOut {
		t.Fatalf("Memnite stayed phased out after The Pandorica untapped: %+v", o)
	}
	replayCheck(t, e, cfg)
}
