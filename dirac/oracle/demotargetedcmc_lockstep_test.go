package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The demo Targeted$CardManaCost pool (dirac/cardset/pools/fixtures/demotargetedcmc):
// Behemoth of Vault 0's ImmediateTrigger asks UnlessCost$ PayEnergy<X> with
// SVar:X Targeted$CardManaCost before its body declares ValidTgts$, and
// Kozilek, the Great Distortion's AB$ Counter lowers Cost$
// Discard<1/Card.cmcEQX> before the Counter declares its target. The kernel
// reads the chosen target's mana value (targetMV); the lowering and the target
// stage around it are dirac/kgen/lower_targetmv.go and dirac/cost_targetmv.go.
const demoTargetedCmcPool = "demotargetedcmc"

var demoTargetedCmcDecks = [2][]cardset.Entry{
	{
		{Name: "Kozilek, the Great Distortion", Count: 3},
		{Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Worn Powerstone", Count: 4}, {Name: "Thran Dynamo", Count: 3},
		{Name: "Wastes", Count: 22},
	},
	{
		{Name: "Behemoth of Vault 0", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Divination", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Carrion Feeder", Count: 2},
		{Name: "Mountain", Count: 7}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 2},
	},
}

// demoTargetedCmcPolicy casts the two carriers first, then activates Kozilek's
// counter or Carrion Feeder's sacrifice, and otherwise plays the random base
// game with affordable targets.
func demoTargetedCmcPolicy(seed uint64) Policy {
	base := AffordableTargetPolicy(RandomPolicy(seed))
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, name := range []string{"Kozilek, the Great Distortion", "Behemoth of Vault 0"} {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					if f := optionFace(c, a); f != nil && f.Name == name {
						return a
					}
				}
			}
		}
		for _, name := range []string{"Kozilek, the Great Distortion", "Carrion Feeder"} {
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate {
					if f := optionFace(c, a); f != nil && f.Name == name {
						return a
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// demoTargetedCmcGames plays seeds 1..hi of the two-seat matchup and returns
// the clean games, the divergent ones and the target-bound discard reads.
func demoTargetedCmcGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.TargetCmcStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoTargetedCmcPool, demoTargetedCmcDecks)
	st = &dirac.TargetCmcStats{}
	dirac.DebugTargetCmcStats = st
	defer func() { dirac.DebugTargetCmcStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoTargetedCmcPool, demoTargetedCmcDecks, gd, 1, hi, mode, nil, nil, demoTargetedCmcPolicy)
	return c, d, st
}

// TestDemoTargetedCmcLockstep: no divergence and no kernel stall over 64 seeds,
// and across them a target-bound discard read (Kozilek's cost) decided a pick.
func TestDemoTargetedCmcLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	clean, divs, st := demoTargetedCmcGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.CostReads == 0 {
		t.Error("no game read the target-bound discard cost (Kozilek's cmcEQX did not fire)")
	}
	if st.UnlessEnergy == 0 {
		t.Error("no game read an unless energy amount from its target (Behemoth's PayEnergy<X> did not fire)")
	}
	t.Logf("%d clean games, %d target-bound discard reads, %d target-mana-value energy amounts", clean, st.CostReads, st.UnlessEnergy)
}

// TestDemoTargetedCmcMutation: with the target-bound read disabled some game
// diverges from gorge.
func TestDemoTargetedCmcMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTgtMVCost = true
	defer func() { dirac.DebugNoTgtMVCost = false }()
	_, divs, _ := demoTargetedCmcGames(t, 64, lockstepQuiet)
	if len(divs) == 0 {
		t.Fatal("DebugNoTgtMVCost: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoTargetedCmcLockstep: four seats, gorge leading.
func TestFourSeatDemoTargetedCmcLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := demoDeckNames(demoTargetedCmcDecks[0])
	b := demoDeckNames(demoTargetedCmcDecks[1])
	if n := pod4FixtureLockstep(t, demoTargetedCmcPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
