package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Brad Boimler, Eager Ensign lockstep: the tap-triggered until-end-of-turn
// "if one or more counters would be put on a permanent you control, put that
// many plus one of each of those kinds instead" (ExtCounterPlusEOT), beside
// Twiddle to tap without combat, counter makers on both sides (so an
// opponent's permanent is a control), proliferate, removal and vanilla
// creatures. The expected answer at every decision is gorge's.
var kboimlerDecks = [2][]cardset.Entry{
	{
		{Name: "Brad Boimler, Eager Ensign", Count: 4}, {Name: "Twiddle", Count: 4},
		{Name: "Battlegrowth", Count: 4}, {Name: "Servant of the Scale", Count: 4},
		{Name: "Contentious Plan", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Forest", Count: 6},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Battlegrowth", Count: 4}, {Name: "Servant of the Scale", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
}

func TestBoimlerLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "boimler", kboimlerDecks)
	before := dirac.CtrPlusEOTStats.Applied.Load()
	p3Lockstep(t, "boimler", reg, set, decks, tbl, 600)
	if got := dirac.CtrPlusEOTStats.Applied.Load(); got == before {
		t.Fatal("no counter placement got the plus one: the pool does not exercise the flag")
	}
}

// TestBoimlerMutation proves the pool exercises the flag: with it
// ignored, a game that puts counters after Boimler taps diverges from gorge.
func TestBoimlerMutation(t *testing.T) {
	reg, set, decks, tbl := p3Setup(t, "boimler", kboimlerDecks)
	old := dirac.DebugCtrPlusEOTOff
	dirac.DebugCtrPlusEOTOff = true
	defer func() { dirac.DebugCtrPlusEOTOff = old }()
	for seed := uint64(1); seed <= 300; seed++ {
		m, err := NewMatchup(reg, [2]string{"boimler-a", "boimler-b"}, decks, 1)
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
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges without the plus-one flag (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with the plus-one flag ignored: the pool does not exercise it")
}
