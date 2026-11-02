package decision

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestRequiredBlockTeamSearchStaysSmall: must-be-blocked attackers that each need
// a team outnumber what the blockers can make, so no team meets every requirement
// and the search has no early exit. The blockers are interchangeable, and the
// search must not walk each way of dealing them out.
func TestRequiredBlockTeamSearchStaysSmall(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ blockers, attackers, teams int }{
		{9, 8, 4},
		{11, 6, 5},
		{13, 7, 6},
		{20, 12, 10},
		{30, 20, 15},
	} {
		d := &Decision{Kind: KBlockers, Max: tc.blockers}
		for b := 0; b < tc.blockers; b++ {
			for a := 0; a < tc.attackers; a++ {
				d.Options = append(d.Options, Option{Index: len(d.Options), Kind: "block",
					Obj: state.ObjID(100 + b), Attacker: state.ObjID(a + 1), AttackMust: true, MinBlockers: 2})
			}
		}
		got := d.blockRequiredCore()
		if n := d.blockRequirementsSatisfied(got); n != tc.teams || len(got) != 2*tc.teams {
			t.Errorf("%d blockers, %d attackers: %d requirements met by %d blockers, want %d by %d",
				tc.blockers, tc.attackers, n, len(got), tc.teams, 2*tc.teams)
		}
	}
}

// requiredBlockBoard is a board of mixed team minimums where each blocker can
// block only some attackers, so no two attackers are interchangeable.
func requiredBlockBoard(blockers, attackers int) *Decision {
	r := rand.New(rand.NewPCG(1, 2))
	d := &Decision{Kind: KBlockers, Max: blockers}
	for a := 0; a < attackers; a++ {
		minimum := 1 + r.IntN(3)
		for b := 0; b < blockers; b++ {
			if r.IntN(5) < 4 {
				d.Options = append(d.Options, Option{Index: len(d.Options), Kind: "block",
					Obj: state.ObjID(100 + b), Attacker: state.ObjID(a + 1), AttackMust: a%2 == 0, MinBlockers: minimum})
			}
		}
	}
	return d
}

// TestRequiredBlockSearchPastItsBudgetAnswersLegally: the search stops at its
// budget with a legal team, and with more budget finds as many requirements.
func TestRequiredBlockSearchPastItsBudgetAnswersLegally(t *testing.T) {
	t.Parallel()
	d := requiredBlockBoard(20, 12)
	short := d.blockRequiredSearch(1 << 10)
	full := d.blockRequiredCore()
	if !d.blockAnswerLegal(short) || !d.blockAnswerLegal(full) {
		t.Fatalf("an illegal team: %v, %v", short, full)
	}
	if n, m := d.blockRequirementsSatisfied(short), d.blockRequirementsSatisfied(full); n == 0 || n > m {
		t.Fatalf("%d requirements at a small budget, %d at the full one", n, m)
	}
}
