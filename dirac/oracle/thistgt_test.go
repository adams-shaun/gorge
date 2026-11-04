package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The own-target Effect: Wesley Crusher, Starfleet Cadet's enters trigger
// chooses two modes, one of which makes an Effect that remembers that link's
// own target (RememberObjects$ ThisTargetedCard) and keeps it from being
// blocked this turn (CR 700.2, 509.1b). On a link with its own target gorge
// remembers that target, the same object Targeted names there, and the
// kernel lowers both alike (dirac/kgen/lower_zone.go). The fixture pool is
// dirac/cardset/pools/fixtures/thistgt.

var thisTgtDecks = []fixtureDecks{{
	{
		{Name: "Wesley Crusher, Starfleet Cadet", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Wesley Crusher, Starfleet Cadet", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Island", Count: 8},
	},
}}

var thisTgtWatch = map[string]bool{"Wesley Crusher, Starfleet Cadet": true}

func TestThisTgtLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "thistgt", thisTgtDecks, thisTgtWatch, 200, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Wesley Crusher, Starfleet Cadet"] < 60 {
		t.Errorf("only %d Wesley Crusher casts", run.casts["Wesley Crusher, Starfleet Cadet"])
	}
}

// TestThisTgtMutation makes Wesley Crusher himself unblockable instead of the
// creature the mode targets: the lockstep must diverge.
func TestThisTgtMutation(t *testing.T) {
	run := runFixtureLockstep(t, "thistgt", thisTgtDecks, thisTgtWatch, 64, false, func(tbl *kop.Table) {
		mutateCard(t, tbl, "Wesley Crusher, Starfleet Cadet", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpGrantKeyword && in.Obj != kop.RefSelf {
				in.Obj = kop.RefSelf
			}
		})
	})
	run.requireDivergence(t, "the source is made unblockable instead of its target")
}
