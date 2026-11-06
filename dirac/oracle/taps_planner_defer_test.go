package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A lowered Taps watcher classes the mana source its tap touches in gorge's
// payment planner (pay.PaymentPlanAbilityTier): Judge of Currents' gain-life
// body defers the Merfolk {T} dorks (the census never lists them, so autopay
// taps an ordinary source), Hawkeye's Bow's vetted damage to an opponent
// leaves its creature ordinary. The kernel mirrors the class from
// kop.Trigger.TapPlan (mana_tapsdefer.go). The expected answer at every
// decision is gorge's.
var ktapsDeferDecks = [2][]cardset.Entry{
	{
		{Name: "Judge of Currents", Count: 4}, {Name: "Firdoch Core", Count: 3}, {Name: "Birds of Paradise", Count: 4},
		{Name: "Savannah Lions", Count: 5}, {Name: "Hawkeye's Bow", Count: 2},
		{Name: "Plains", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11},
	},
}

func TestTapsPlannerDefer(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsdefer", ktapsDeferDecks, 200, "KTAPSDEFER_SEEDS")
}

// TestTapsPlannerDeferMutation: with the classification dropped some seed of
// the first 40 diverges from gorge, proof the pool exercises it.
func TestTapsPlannerDeferMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapsDefer = true
	defer func() { dirac.DebugNoTapsDefer = false }()
	if !lockstepDiverges(t, "tapsdefer", ktapsDeferDecks, 40) {
		t.Fatal("no seed diverged with the Taps-watcher classification dropped: the pool does not exercise it")
	}
}
