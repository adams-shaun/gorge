package decision

import (
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
