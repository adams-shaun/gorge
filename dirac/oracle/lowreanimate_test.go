package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A destroy-then-return with an end-step sacrifice (CR 603.7): Come Back
// Wrong destroys a creature, returns it from the graveyard under the
// caster's control and sacrifices it at the next end step. The fixture pool
// is dirac/cardset/pools/fixtures/lowreanimate: creature cards (returned) and
// Raise the Alarm's tokens (which never reach a graveyard to return from).

var lowReanimateDecks = []fixtureDecks{{
	{
		{Name: "Come Back Wrong", Count: 10}, {Name: "Walking Corpse", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Come Back Wrong", Count: 4}, {Name: "Raise the Alarm", Count: 6}, {Name: "Savannah Lions", Count: 6},
		{Name: "Giant Spider", Count: 4}, {Name: "Swamp", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
	},
}}

var lowReanimateWatch = map[string]bool{"Come Back Wrong": true}

func TestLowReanimateLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "lowreanimate", lowReanimateDecks, lowReanimateWatch, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Come Back Wrong"] < 100 {
		t.Errorf("only %d Come Back Wrong casts", run.casts["Come Back Wrong"])
	}
}

// TestLowReanimateMutation drops the end-step promise: the returned creature
// stays, so the lockstep must diverge at the end step.
func TestLowReanimateMutation(t *testing.T) {
	run := runFixtureLockstep(t, "lowreanimate", lowReanimateDecks, lowReanimateWatch, 64, false, func(tbl *kop.Table) {
		mutateCard(t, tbl, "Come Back Wrong", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpAtEOT {
				in.Op = kop.OpNop
			}
		})
	})
	run.requireDivergence(t, "the returned creature is never sacrificed")
}
