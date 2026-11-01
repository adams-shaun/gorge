package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// cleanupScen: p0's Hill Giant with three -1/-1 counters is kept alive by
// an until-end-of-turn +0/+3; the game is passed to the cleanup step.
func cleanupScen(t *testing.T, cfg Config) (*scen, Obj, int) {
	sc := newScen(t, cfg)
	g := sc.perm(v.HillGiant, 0)
	p := &sc.s.perms[sc.slot(g)]
	p.setCtrN(sc.s, int(kop.CtrM1M1), 3)
	p.eotT = 3
	turn := int(sc.s.turn)
	for i := 0; i < 40 && sc.s.step != kop.StepCleanup; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			sc.s.Apply(d.Options[d.N-1])
			continue
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	return sc, g, turn
}

// TestCleanupChecksStateBasedActions: CR 514.3a: the
// cleanup step's state-based check follows the end of "until end of turn"
// effects, so the Giant dies in that cleanup step and the active player
// then gets priority.
func TestCleanupChecksStateBasedActions(t *testing.T) {
	sc, g, turn := cleanupScen(t, Config{NoElide: true})
	d := sc.s.Step()
	if d.Kind != DkPriority || sc.s.step != kop.StepCleanup || int(sc.s.turn) != turn || d.Player != sc.s.active {
		t.Fatalf("want the active player's priority in turn %d's cleanup, got kind %d step %d turn %d player %d", turn, d.Kind, sc.s.step, sc.s.turn, d.Player)
	}
	if sc.onBF(g) {
		t.Fatal("the 0/0 Giant is still on the battlefield in the cleanup step")
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	if d = sc.s.Step(); d.Kind != DkPriority || sc.s.step != kop.StepCleanup {
		t.Fatalf("want the other player's pass in the cleanup step, got kind %d step %d", d.Kind, sc.s.step)
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Step()
	if int(sc.s.turn) != turn+1 {
		t.Fatalf("the repeated cleanup did not end the turn (turn %d)", sc.s.turn)
	}
}
