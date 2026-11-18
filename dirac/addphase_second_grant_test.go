package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// addPhaseSecondWalk plays the turn's step walk from Main1 as advance does for
// the steps it visits (an empty combat jumps Attackers to EndCombat), calling
// grant on arriving at each step, and returns the visited steps up to the end
// step. A grant is a Relentless Assault resolving: ExtAddPhase of the
// CombatMain kind, one phase pair, at the step it resolves in.
func addPhaseSecondWalk(t *testing.T, s *State, k *prep, grant func(step kop.Step, visit int) int) []kop.Step {
	t.Helper()
	s.step = kop.StepMain1
	seen := map[kop.Step]int{}
	var got []kop.Step
	for len(got) < 40 {
		got = append(got, s.step)
		if s.step == kop.StepEnd {
			return got
		}
		seen[s.step]++
		for i := grant(s.step, seen[s.step]); i > 0; i-- {
			s.addPhaseGrant(k, &kop.Instr{B: kop.Const(1), C: kop.AddPhaseCombatMain})
		}
		if s.g.stalled != 0 {
			t.Fatalf("grant at %v stalled: code %d (walk so far %v)", s.step, s.g.stalled, got)
		}
		next := s.step + 1
		if s.step == kop.StepAttackers {
			next = kop.StepEndCombat
		}
		s.step = s.addPhaseNext(next)
	}
	t.Fatalf("the walk did not reach the end step: %v", got)
	return nil
}

// TestAddPhaseSecondGrant: a second CombatMain grant (Relentless Assault)
// resolving beside a pending one queues behind it, as gorge's extraPhaseBoundary
// does (MOST RECENT pending grant at the splice point first; every completed
// grant's FollowedBy$ main, or in the second main the combat's own resume point,
// then the next pending grant spliced at the same point). Each want is gorge's
// step order for the same grants.
func TestAddPhaseSecondGrant(t *testing.T) {
	const (
		m1 = kop.StepMain1
		bc = kop.StepBeginCombat
		at = kop.StepAttackers
		ec = kop.StepEndCombat
		m2 = kop.StepMain2
		en = kop.StepEnd
	)
	cases := []struct {
		name  string
		grant func(step kop.Step, visit int) int
		want  []kop.Step
	}{
		{"two cast in the first main",
			func(step kop.Step, visit int) int {
				if step == m1 {
					return 2
				}
				return 0
			},
			// each grant: combat, then its additional main before the ordinary combat
			[]kop.Step{m1, bc, at, ec, m2, bc, at, ec, m2, bc, at, ec, m2, en}},
		{"two cast in the second main",
			func(step kop.Step, visit int) int {
				if step == m2 && visit == 1 {
					return 2
				}
				return 0
			},
			// the first completed combat hands to the second grant at once; the main follows both
			[]kop.Step{m1, bc, at, ec, m2, bc, at, ec, bc, at, ec, m2, en}},
		{"one in the first main, two in its additional main",
			func(step kop.Step, visit int) int {
				switch {
				case step == m1:
					return 1
				case step == m2 && visit == 1:
					return 2
				}
				return 0
			},
			[]kop.Step{m1, bc, at, ec, m2, bc, at, ec, bc, at, ec, m2, en}},
		{"two in the first main, a third in the first additional main",
			func(step kop.Step, visit int) int {
				switch {
				case step == m1:
					return 2
				case step == m2 && visit == 1:
					return 1
				}
				return 0
			},
			// the newest grant takes the splice point; the older Main1 grant is never spliced
			[]kop.Step{m1, bc, at, ec, m2, bc, at, ec, m2, en}},
		{"one in the first main alone",
			func(step kop.Step, visit int) int {
				if step == m1 {
					return 1
				}
				return 0
			},
			[]kop.Step{m1, bc, at, ec, m2, bc, at, ec, m2, en}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newScen(t, Config{})
			got := addPhaseSecondWalk(t, sc.s, sc.k, c.grant)
			if !slices.Equal(got, c.want) {
				t.Fatalf("steps\n got  %v\n want %v", got, c.want)
			}
		})
	}
}
