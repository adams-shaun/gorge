package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Lamplight Phoenix's die-pay-return lockstep (CR 701.59a, 603.4): "When it
// dies, you may exile it and collect evidence 4. If you do, return it to the
// battlefield tapped." The trigger's own cost exiles the Phoenix itself, and
// gorge's Defined$ ExiledCards (cumulative.go settleTriggeredMandatory) names
// it for the return: after a paid cost it must be back on the battlefield,
// with the graveyard in the same order, at the same decision in both engines.
// The Phoenix-heavy deck dies to the other deck's burn and removal, which also
// fill the graveyards the evidence is taken from.
var klamplightphoenixMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Lamplight Phoenix", Count: 8},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 20},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}}

func TestLamplightPhoenixReturnLockstep(t *testing.T) {
	t.Parallel()
	var tal kcollectevidencexTally
	sonRun(t, "collectevidencex", klamplightphoenixMatchups, 128, true, nil, kcollectevidencexCheck(&tal))
	t.Logf("%d games, %d Phoenix evidence picks", tal.games, tal.phxPick)
	if tal.phxPick < 5 {
		t.Errorf("the pool barely pays the Phoenix's evidence: %d picks (want >= 5)", tal.phxPick)
	}
}
