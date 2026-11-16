package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// scratch probe: the lockstep game's decision 328 board — gorge offers
// cast(Don Andres), the kernel withholds. Is the offer real?
func TestZZDAProbe(t *testing.T) {
	e, spell := probeBoardHand(t, "Don Andres, the Renegade", []string{"Swamp", "Swamp", "Karakas", "Henge of Ramos", "Crossroads Candleguide", "Island"})
	for _, n := range []string{"Satyr Firedancer", "Silent Specter", "Mercadian Bazaar", "Death of a Thousand Stings", "Molting Harpy"} {
		o := e.G.AddObject(corpusAlternativeCard(t, n), 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZHand})
	}
	// the game's island was tapped; tap it via a mana activation is overkill —
	// leave untapped and note it.
	e.priorityRound()
	if e.Pending() == nil {
		t.Fatalf("precondition: no priority ask posed")
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == spell {
			t.Errorf("Don Andres cast: reason=%q detail=%q plan=%v scriptSteps=%d", pp.Reason, pp.Detail, pp.Plan != nil, len(pp.Script))
			for i, st := range pp.Script {
				t.Errorf("  step %d: kind=%v picks=%v", i, st.Kind, st.Picks)
			}
		}
	}
	var castOpt bool
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == spell {
			castOpt = true
		}
	}
	t.Errorf("pool=%v castOption=%v", e.G.Players[0].Pool, castOpt)
}
