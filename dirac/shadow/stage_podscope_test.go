package shadow_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// podBlockInput is a four-seat observation at the declare blockers step: seat
// 0's Savannah Lions (object 0) attacks seat 1 and is blocked by the Lions of
// seat `blocker` (a hidden-hand card moved to its battlefield).
func podBlockInput(blocker int) *shadow.Input {
	in := reasonsInput()
	for p := 2; p < 4; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: 20, HandSize: 7, LibrarySize: 13}
		for i := 0; i < 7; i++ {
			in.Hidden.Hand[p] = append(in.Hidden.Hand[p], shadow.Card{Obj: dirac.Obj(p*20 + i), Name: "Plains"})
		}
		for i := 0; i < 13; i++ {
			in.Hidden.Library[p] = append(in.Hidden.Library[p], shadow.Card{Obj: dirac.Obj(p*20 + 7 + i), Name: "Savannah Lions"})
		}
		in.View.Players = append(in.View.Players, pv)
	}
	seat1 := state.PlayerID(1)
	obj := state.ObjID(blocker * 20)
	in.View.Step = "declare-blockers"
	in.View.Players[0].Hand = in.View.Players[0].Hand[1:]
	in.View.Players[0].HandSize--
	in.View.Players[0].Battlefield = []view.CardView{{ID: 0, Name: "Savannah Lions", Attacking: true, AttackingPlayer: &seat1, Blocked: true, BlockedBy: []state.ObjID{obj}}}
	in.Hidden.Hand[blocker] = in.Hidden.Hand[blocker][1:]
	in.View.Players[blocker].HandSize--
	in.View.Players[blocker].Battlefield = []view.CardView{{ID: obj, Name: "Savannah Lions", Owner: state.PlayerID(blocker), Controller: state.PlayerID(blocker)}}
	return in
}

// TestStagePodBlockRules: at more than two seats, blocks shown to an observer
// do not settle the declare blockers step (another defender may still be to
// declare), and a blocker must be controlled by the seat its attacker attacks.
func TestStagePodBlockRules(t *testing.T) {
	t.Parallel()
	// the control: the attacked seat's blocker, a decision shown (the priority
	// round after the declarations), stages in the pod
	in := podBlockInput(1)
	in.View.Decision = &decision.Decision{Kind: decision.KPriority}
	if _, err := shadow.StageFromView(in); err != nil {
		t.Fatalf("the attacked seat's block with priority shown does not stage: %v", err)
	}
	// the same block, no decision shown: an observer at four seats
	in = podBlockInput(1)
	var u dirac.ErrUnstageable
	if _, err := shadow.StageFromView(in); !errors.As(err, &u) || u.Reason != shadow.ReasonDecision {
		t.Errorf("an observer at a four-seat declare blockers step: want %q, got %v", shadow.ReasonDecision, err)
	}
	// a block by a seat the attacker does not attack
	in = podBlockInput(2)
	in.View.Decision = &decision.Decision{Kind: decision.KPriority}
	if _, err := shadow.StageFromView(in); !errors.As(err, &u) || u.Reason != dirac.ReasonCombat {
		t.Errorf("a blocker of a seat the attacker does not attack: want %q, got %v", dirac.ReasonCombat, err)
	}
}

// TestStageBlockersStepNeedsAnAttacker: the declare blockers step with no
// attacker on the board means every attacker left combat, and a blocker of a
// departed attacker keeps blocking with no mark a view shows, so the position
// is not staged, at two seats or in a pod.
func TestStageBlockersStepNeedsAnAttacker(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]*shadow.Input{"two seats": reasonsInput(), "four seats": podBlockInput(1)} {
		in.View.Step = "declare-blockers"
		in.View.Decision = &decision.Decision{Kind: decision.KPriority}
		if name == "four seats" {
			in.View.Players[0].Battlefield[0].Attacking, in.View.Players[0].Battlefield[0].Blocked = false, false
			in.View.Players[0].Battlefield[0].AttackingPlayer, in.View.Players[0].Battlefield[0].BlockedBy = nil, nil
		}
		var u dirac.ErrUnstageable
		if _, err := shadow.StageFromView(in); !errors.As(err, &u) || u.Reason != dirac.ReasonCombat {
			t.Errorf("%s: want %q, got %v", name, dirac.ReasonCombat, err)
		}
	}
}
