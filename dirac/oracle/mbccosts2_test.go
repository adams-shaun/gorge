package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The MBC costs-2 lowerings in lockstep with gorge: each spelling plays one of
// its cards from a fixture pool (dirac/cardset/pools/fixtures/mbc*).
//
//   - kw:Entwine (CR 702.42): Unbounded Potential, Stir the Pride, Grab the
//     Reins, Betrayal of Flesh (Entwine: Sac<3/Land>), Goblin War Party. gorge
//     offers an "entwined" cast (base + entwine cost) that announces every
//     mode with a legal target; the kernel's kicked cast is that option
//     (dirac/kw_entwine.go).

var mbcEntwineWatch = map[string]bool{
	"Unbounded Potential": true, "Stir the Pride": true, "Grab the Reins": true,
	"Betrayal of Flesh": true, "Goblin War Party": true,
}

var mbcEntwineDecks = []fixtureDecks{{
	{
		{Name: "Unbounded Potential", Count: 3}, {Name: "Stir the Pride", Count: 3}, {Name: "Goblin War Party", Count: 3},
		{Name: "Grab the Reins", Count: 3}, {Name: "Betrayal of Flesh", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Unbounded Potential", Count: 2}, {Name: "Stir the Pride", Count: 2}, {Name: "Goblin War Party", Count: 4},
		{Name: "Grab the Reins", Count: 4}, {Name: "Betrayal of Flesh", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
	},
}}

func TestMBCCosts2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("kw:Entwine", func(t *testing.T) {
		run := runFixtureLockstep(t, "mbcentwine", mbcEntwineDecks, mbcEntwineWatch, 120, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		for n := range mbcEntwineWatch {
			if run.casts[n] < 10 {
				t.Errorf("%s cast %d times", n, run.casts[n])
			}
		}
	})
}

// TestMBCEntwineMutation: an entwined cast that picks one mode like a plain
// one must diverge (gorge announces every mode with a legal target).
func TestMBCEntwineMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEntwinePickOne = true
	defer func() { dirac.DebugEntwinePickOne = false }()
	run := runFixtureLockstep(t, "mbcentwine", mbcEntwineDecks, mbcEntwineWatch, 120, false, nil)
	run.requireDivergence(t, "an entwined cast picks one mode")
}
