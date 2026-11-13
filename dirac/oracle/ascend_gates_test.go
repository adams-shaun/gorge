package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The blessing-gate lowering (CR 702.131): the corpus carriers the gate
// lowering lifted past the four spellings kgen refused — the trigger
// Blessing$ True (Deadeye Brawler, Mausoleum Harpy, Resplendent Griffin,
// Twilight Prophet), the sub-ability Condition$ Blessing (the Golden City of
// Orazca stays out: its token script and ChaosEnsues trigger refuse), the
// Mode$ CantBlockBy static Condition$ Blessing (Slippery Scoundrel) and the
// Activation$ Blessing (Arch of Orazca, Orazca Relic) — beside the same
// blessing fillers the ascend pool plays. Every gate is the controller's
// city's-blessing latch (gorge trigger_condition.go's PKBlessing arm,
// effects/conditions.go's bare-Blessing arm, layers_static.go's condBlessing,
// legal_activation.go's activationConditionOKBlessing), read as it triggers,
// as the node resolves, as the static is consulted and as the ability is
// offered, against the same ctx.ctrl the kernel's CondBlessing evaluates
// (dirac/kw_ascend.go condBlessing). The expected answer at every decision is
// gorge's.

var kascendGateMatchups = [2][]cardset.Entry{
	{ // A: W/U/B, the seven lifted gate carriers plus the blessing fillers
		{Name: "Deadeye Brawler", Count: 2}, {Name: "Mausoleum Harpy", Count: 2},
		{Name: "Resplendent Griffin", Count: 2}, {Name: "Twilight Prophet", Count: 2},
		{Name: "Slippery Scoundrel", Count: 2}, {Name: "Arch of Orazca", Count: 2},
		{Name: "Orazca Relic", Count: 2}, {Name: "Spire Winder", Count: 3},
		{Name: "Secrets of the Golden City", Count: 2}, {Name: "Radiant Destiny", Count: 2},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
	},
	{ // B: five-colour vanilla beaters, no Ascend: the deck the gates read
		// against — its creatures attack (the Scoundrel's CantBlockBy gate) and
		// die to A's blocks and to each other (the Harpy's Dies gate)
		{Name: "Savannah Lions", Count: 3}, {Name: "Goblin Piker", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Wind Drake", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 5},
	},
}

// kascendGateLifted is every carrier the gate lowering lifted that the
// matchup plays.
var kascendGateLifted = []string{
	"Arch of Orazca", "Deadeye Brawler", "Mausoleum Harpy", "Orazca Relic",
	"Resplendent Griffin", "Slippery Scoundrel", "Twilight Prophet",
}

// TestAscendBlessingGates: every decision of every seed matches gorge; every
// lifted card was played; the latch granted (both forms) and the gates read
// it both ways. With the latch disabled (dirac.DebugBlessingOff) some seed
// diverges, so the gates are load-bearing.
func TestAscendBlessingGates(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.BlessStats{}
	dirac.DebugBlessStats = st
	defer func() { dirac.DebugBlessStats = nil }()
	probe := &kascendProbe{played: map[string]int{}}
	policy := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				probe.observe(c.G)
			}
			return base.Pick(c)
		})
	}
	const seeds = 40
	if div := lockstepRunPolicy(t, "ascend", kascendGateMatchups, seeds, true, nil, nil, policy); div != 0 {
		t.Errorf("%d of %d seeds diverged", div, seeds)
	}
	t.Logf("kernel: %+v; gorge blessed at %d seat-decisions", *st, probe.blessed)
	if probe.blessed == 0 {
		t.Error("gorge never granted the blessing: the pool never reached ten permanents")
	}
	if st.Permanent == 0 {
		t.Error("permanent-form grants (CR 702.131b): none")
	}
	// The spell form (CR 702.131a) is not asserted here: every gate carrier
	// is a permanent, so deck A's permanent form latches before any of its
	// Ascend spells resolves and a grant only counts for an unblessed seat
	// (kw_ascend.go spellBlessing); the spell-only deck asserts it in
	// TestAscendBlessingLowers.
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.CondYes, "blessing gates that held"},
		{st.CondNo, "blessing gates that failed"},
	} {
		if c.n == 0 {
			t.Errorf("%s: none", c.what)
		}
	}
	for _, n := range kascendGateLifted {
		if probe.played[n] == 0 {
			t.Errorf("%s: never played", n)
		}
	}
	if t.Failed() {
		return
	}
	// the mutation: with the latch off, the gates must expose it
	dirac.DebugBlessingOff = true
	defer func() { dirac.DebugBlessingOff = false }()
	if !lockstepDiverges(t, "ascend", kascendGateMatchups, seeds) {
		t.Error("no seed diverged with the blessing latch disabled: the pool never reads a granted blessing")
	}
}
