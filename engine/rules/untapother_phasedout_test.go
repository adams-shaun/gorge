package rules

// CR 702.26b -- a phased-out permanent is treated as though it does not
// exist, so its UntapOtherPlayer static untaps nothing during another
// player's untap step.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestPhasedOutUntapOtherStaticDoesNotUntap(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		phaseOut bool
	}{
		{name: "phased in", phaseOut: false},
		{name: "phased out", phaseOut: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cfg := phasesGame(t, 702, "Seedborn Muse", "Grizzly Bears")
			bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
			muse := moveSeededCard(t, e, 0, cr702corpusCard(t, "Seedborn Muse"), state.ZBattlefield)
			e.emit(events.Event{Kind: events.Tap, Obj: bears})
			if tc.phaseOut {
				e.emit(events.Event{Kind: events.PhaseOut, Obj: muse, Amount: 1})
			}
			if o := e.G.Obj(bears); o == nil || !o.Tapped || o.PhasedOut {
				t.Fatalf("precondition: the Bears are not tapped and phased in: %+v", o)
			}
			if o := e.G.Obj(muse); o == nil || o.PhasedOut != tc.phaseOut {
				t.Fatalf("precondition: Seedborn Muse PhasedOut is not %v: %+v", tc.phaseOut, o)
			}
			if got := e.untapOtherStaticsMatch(bears); got == tc.phaseOut {
				t.Errorf("CR 702.26b: untapOtherStaticsMatch = %v with the Muse phased out = %v", got, tc.phaseOut)
			}
			addMana(t, e, 0, "")
			// Seat 1's turn: seat 0 has no untap step of its own before it,
			// so the Muse cannot phase back in first.
			driveToStepAll(t, e, e.G.Turn+1, 1, state.StepUpkeep)
			if o := e.G.Obj(muse); o == nil || o.PhasedOut != tc.phaseOut {
				t.Fatalf("precondition: Seedborn Muse PhasedOut changed before the check: %+v", o)
			}
			if got := e.G.Obj(bears).Tapped; got != tc.phaseOut {
				t.Fatalf("CR 702.26b: after seat 1's untap step Bears tapped = %v, want %v", got, tc.phaseOut)
			}
			replayCheck(t, e, cfg)
		})
	}
}
