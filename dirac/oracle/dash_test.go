package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Dash (CR 702.109a): Ragavan, Nimble Pilferer's hand cast for {1}{R} gives
// the permanent haste until end of turn and returns it to its owner's hand at
// the beginning of the next end step. gorge offers the "dashed" cast beside
// the plain one (a potential play), grants the UntilEOT haste as the
// permanent enters and registers the incarnation-tracked __kwDashReturn; the
// kernel prices CardDef.DashCost, marks the spell entry, and registers the
// same haste and delayed return as it enters (dirac/kw_dash.go). The fixture
// pool is dirac/cardset/pools/fixtures/dash.

var dashDecks = []fixtureDecks{{
	{
		{Name: "Ragavan, Nimble Pilferer", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Ragavan, Nimble Pilferer", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 2}, {Name: "Mountain", Count: 14}, {Name: "Plains", Count: 6},
	},
}}

var dashWatch = map[string]bool{"Ragavan, Nimble Pilferer": true}

func TestDashLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "dash", dashDecks, dashWatch, 150, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Ragavan, Nimble Pilferer"] < 60 {
		t.Errorf("only %d Ragavan casts", run.casts["Ragavan, Nimble Pilferer"])
	}
}

// TestDashMutation: skipping the haste grant, or the end-step return, of a
// dashed permanent must each make the lockstep diverge (gorge gives a dashed
// Ragavan haste and returns it to hand at the end step).
func TestDashMutation(t *testing.T) {
	for _, m := range []struct {
		what string
		flag *bool
	}{{"a dashed permanent gains no haste", &dirac.DebugDashNoHaste}, {"a dashed permanent is not returned", &dirac.DebugDashNoReturn}} {
		t.Run(m.what, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			run := runFixtureLockstep(t, "dash", dashDecks, dashWatch, 150, false, nil)
			run.requireDivergence(t, m.what)
		})
	}
}
