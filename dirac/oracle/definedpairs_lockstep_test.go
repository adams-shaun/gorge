package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Defined$ A & B composition lockstep (kgen lower_defineduniononplayers.go,
// lower_definedset.go setUnionObjects): Black Widow, Intel Expert's
// combat-damage trigger draws for "You & TriggeredTarget" (two Draw nodes in
// script order); Alaborn Zealot and Loyal Sentry destroy
// "TriggeredAttackerLKICopy & Self" (two Destroy nodes). No corpus card
// with an object union plays yet: Dragon Turtle's union lowers, but the
// builder refuses its kop SkipNextUntap a set subject (a kernel op), and
// Leaf-Leap Guide's Pump refuses Duration$ Perpetual. The pool is
// dirac/cardset/pools/fixtures/definedpairs.
const definedPairsPool = "definedpairs"

const blackWidow = "Black Widow, Intel Expert"

var definedPairsDecks = [2][]cardset.Entry{
	{
		{Name: blackWidow, Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Glory Seeker", Count: 4},
		{Name: "Plains", Count: 20},
	},
	{
		{Name: "Alaborn Zealot", Count: 6}, {Name: "Loyal Sentry", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// definedPairsRuns are what the games ran: the kernel's set reads and the
// gorge resolutions of Black Widow's pair trigger.
type definedPairsRuns struct {
	sets      dirac.SetStats
	pairNodes int
}

// definedPairsGames plays seeds 1..hi on the fixture pool (lockstepPool,
// castFirstPolicy, lockstepSeedsMode's seed shapes) and counts, through the
// gorge side's coverage, every resolution of the player pair's trigger.
func definedPairsGames(t *testing.T, hi uint64, mode lockstepMode, mutate func(*kop.Table)) (clean int, divs []lockstepDivergence, runs *definedPairsRuns) {
	t.Helper()
	reg, set, gd := lockstepPool(t, definedPairsPool, definedPairsDecks)
	runs = &definedPairsRuns{}
	dirac.DebugSetStats = &runs.sets
	defer func() { dirac.DebugSetStats = nil }()
	tbl, _, _, err := set.Matchup(definedPairsDecks[0], definedPairsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := uint64(1); seed <= hi; seed++ {
		m, err := NewMatchup(reg, [2]string{definedPairsPool + "-a", definedPairsPool + "-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Finish: func(g *GorgeSide) {
			if cc := CoverageOf(g, m).Cards[blackWidow]; cc != nil {
				for _, n := range cc.Used {
					runs.pairNodes += n
				}
			}
		}}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			from := max(d.At-12, 0)
			msg := fmt.Sprintf("seed %d last actions %v\nseed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, actionStrings(res.Actions[from:d.At]), seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			divs = append(divs, lockstepDivergence{seed, msg})
			if mode == lockstepFailEach {
				t.Error(msg)
			}
			continue
		}
		clean++
	}
	return clean, divs, runs
}

// TestDefinedPairLockstep: no divergence and no kernel stall over 64 seeds,
// and across them the player pair's trigger ran (the set counts are logged:
// no card of the pool reads a set).
func TestDefinedPairLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	clean, divs, runs := definedPairsGames(t, 64, lockstepFailEach, nil)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if runs.pairNodes == 0 {
		t.Error("no game resolved the player pair's trigger")
	}
	t.Logf("%d clean games; sets %+v; pair resolutions %d", clean, runs.sets, runs.pairNodes)
}

// TestDefinedPairMutation: with each player pair's second node acting on the
// first node's player (Black Widow's opponent draws nothing, you draw four),
// some game diverges from gorge, so the pair's two nodes reach the game.
// Swapping the two nodes instead cannot diverge on this pool:
// two Draws of different libraries with nothing between them reach the same
// decisions in either order.
func TestDefinedPairMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	rewritten := 0
	_, divs, runs := definedPairsGames(t, 64, lockstepQuiet, func(tbl *kop.Table) {
		for ci := range tbl.Cards {
			if tbl.Cards[ci].Name != blackWidow {
				continue
			}
			for ai := range tbl.Cards[ci].Abilities {
				prog := tbl.Cards[ci].Abilities[ai].Prog
				for i := 0; i+1 < len(prog); i++ {
					if prog[i].Op == kop.OpDraw && prog[i+1].Op == kop.OpDraw && prog[i].Obj != prog[i+1].Obj {
						prog[i+1].Obj = prog[i].Obj
						rewritten++
					}
				}
			}
		}
	})
	if rewritten == 0 {
		t.Fatal("no player pair of two Draw nodes in the table: the mutation changed nothing")
	}
	if runs.pairNodes == 0 {
		t.Fatal("no game resolved the player pair's trigger")
	}
	if len(divs) == 0 {
		t.Fatal("a pair collapsed onto one player: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}
