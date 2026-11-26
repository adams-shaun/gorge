package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAddPhaseFirstDrawSkipsOnlyTheDraw: CR 103.8a skips the first turn's draw
// step only. Turn 1 of a two-player game, an extra beginning phase (Shadow of
// the Second Sun, spliced after Main2) reaching its draw step completes and
// resumes at the end step, as gorge's skipFirstDrawLanding does; the ordinary
// beginning phase still goes to Main1.
func TestAddPhaseFirstDrawSkipsOnlyTheDraw(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	s.turn = 1
	if s.nSeats() != 2 {
		t.Fatalf("fixture: %d seats, want 2", s.nSeats())
	}
	s.step = kop.StepDraw
	if got := s.firstDrawLanding(); got != kop.StepMain1 {
		t.Fatalf("ordinary draw step lands at %v, want Main1", got)
	}
	s.step = kop.StepMain2
	s.addPhaseGrant(sc.k, &kop.Instr{B: kop.Const(1), C: kop.AddPhaseBeginning})
	if s.g.stalled != 0 || s.g.nxph != 1 {
		t.Fatalf("fixture: grant not queued (stalled %d, nxph %d)", s.g.stalled, s.g.nxph)
	}
	s.step = s.addPhaseNext(kop.StepEnd) // leaving Main2: the extra beginning phase is entered
	if s.step != kop.StepUntap {
		t.Fatalf("fixture: extra phase entered at %v, want Untap", s.step)
	}
	s.step = kop.StepDraw
	if got := s.firstDrawLanding(); got != kop.StepEnd {
		t.Fatalf("extra beginning phase's draw lands at %v, want End", got)
	}
	if s.g.nxph != 0 {
		t.Fatalf("the completed grant stays queued: nxph %d", s.g.nxph)
	}
}
