package dirac

import "testing"

// pendTapCount groups a path's queued triggers by the tap they came from: the
// same triggering object means the same tap. NoObj is the "names no object"
// sentinel, which cannot be told from another tap's, so it falls back to every
// one of the player's triggers; object 0 is a real object index and shares
// only a tap on object 0.
func TestPendTapCountGroupsTapsByTriggeringObject(t *testing.T) {
	var s State
	s.npend = 5
	s.pend[0] = pendingTrig{ctrl: 0, trigObj: NoObj}
	s.pend[1] = pendingTrig{ctrl: 0, trigObj: 0}
	s.pend[2] = pendingTrig{ctrl: 0, trigObj: 7}
	s.pend[3] = pendingTrig{ctrl: 0, trigObj: 7}
	s.pend[4] = pendingTrig{ctrl: 1, trigObj: 7}

	if got, want := s.pendCount(0), 4; got != want {
		t.Fatalf("precondition: pendCount(0) = %d, want %d", got, want)
	}
	if got, want := s.pendTapCount(0, 0), 4; got != want {
		t.Errorf("NoObj trigger counted %d of player 0's triggers, want all 4", got)
	}
	if got, want := s.pendTapCount(0, 1), 1; got != want {
		t.Errorf("trigger on object 0 counted %d, want 1 (object 0 is not NoObj; a no-object trigger and another tap's trigger are not the same tap)", got)
	}
	if got, want := s.pendTapCount(0, 2), 2; got != want {
		t.Errorf("trigger on object 7 counted %d, want 2 (its own tap)", got)
	}
	if got, want := s.pendTapCount(1, 4), 1; got != want {
		t.Errorf("player 1's trigger on object 7 counted %d, want 1", got)
	}
}
