package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// {S} activation costs (CR 107.4h): Boreal Centaur, Icehide Troll, Thermopod,
// Frost Raptor and Phyrexian Ironfoot ask snow mana, which only mana from a
// snow source pays (CR 106.3a). The fixture pool is
// dirac/cardset/pools/fixtures/snowcost.

var snowCostDecks = []fixtureDecks{{
	{
		{Name: "Boreal Centaur", Count: 4}, {Name: "Icehide Troll", Count: 4}, {Name: "Thermopod", Count: 3},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Coldsteel Heart", Count: 2}, {Name: "Snow-Covered Forest", Count: 7},
		{Name: "Snow-Covered Mountain", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Frost Raptor", Count: 4}, {Name: "Phyrexian Ironfoot", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Snow-Covered Island", Count: 6}, {Name: "Snow-Covered Mountain", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
	},
}}

var snowCostWatch = map[string]bool{
	"Boreal Centaur": true, "Icehide Troll": true, "Thermopod": true, "Frost Raptor": true, "Phyrexian Ironfoot": true,
}

func snowCostRun(t *testing.T, report bool, mutate func(*kop.Table)) fixtureRun {
	t.Helper()
	return runFixtureLockstepPolicy(t, "snowcost", snowCostDecks, 120, report, mutate, func(seed uint64, casts map[string]int) Policy {
		return snowWatchPolicy(seed, snowCostWatch, casts)
	})
}

// TestSnowCostLockstep: the kernel and gorge offer the same plays over the
// five {S} carriers. gorge never plans a {S} cost (PlanCostDetail cost:snow),
// so its activation is offered only when the floating pool pays it, and the
// oracle's autopay surface never floats mana at a canonical decision (the
// kernel has no manual mana; gorge's mana abilities are no options, doc.go).
// The games therefore show the two engines agree on never offering the
// activation, with the kernel's gate (snowPays) on; the payment itself is
// TestSnowCostPays in package dirac (the {S} pips paid by the lockstep stay 0).
func TestSnowCostLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.SnowStats{}
	dirac.DebugSnowStats = st
	defer func() { dirac.DebugSnowStats = nil }()
	run := snowCostRun(t, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, snow %+v", run.games, run.divs, run.stalls, run.casts, *st)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls", run.divs, run.stalls)
	}
	for n := range snowCostWatch {
		if run.casts[n] == 0 {
			t.Errorf("no %s was cast in %d games", n, run.games)
		}
	}
}
