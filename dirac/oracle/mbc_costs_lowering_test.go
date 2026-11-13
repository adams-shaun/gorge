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

//   - cost:ExileFromGrave (CR 601.2b): Skeletal Scrying's X B ExileFromGrave<X/Card>.
//     gorge announces X bounded by the mana and the graveyard's cards, then
//     asks exactly X graveyard cards to exile; the kernel's X ask is
//     min-clamped by the candidates and its cost-pick stage walks X picks
//     (dirac/cost_exilex.go).

var mbcExileXWatch = map[string]bool{"Skeletal Scrying": true}

var mbcExileXDecks = []fixtureDecks{{
	{
		{Name: "Skeletal Scrying", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Terror", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Skeletal Scrying", Count: 4}, {Name: "Terror", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5},
	},
}}

func TestMBCCosts2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("cost:ExileFromGrave", func(t *testing.T) {
		run := runFixtureLockstep(t, "mbcexilex", mbcExileXDecks, mbcExileXWatch, 120, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		if run.casts["Skeletal Scrying"] < 10 {
			t.Errorf("Skeletal Scrying cast %d times", run.casts["Skeletal Scrying"])
		}
	})
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
