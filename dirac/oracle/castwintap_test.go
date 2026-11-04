package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The cast window (CR 601.2g) lists a Springleaf Drum in battlefield order and takes the
// first source that helps, so gorge activates it where it stands and asks which creature to
// tap. Decks of hybrid, convoke and either-or-cost spells (Eclipsed Merrow and Boggart,
// Sun-Dappled Celebrant, Wild Unraveling), several Drums and bodies to tap, played over
// many seeds from a random policy that answers every pick.
var castWinTapDecks = [][2][]cardset.Entry{{
	{
		{Name: "Eclipsed Merrow", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Wild Unraveling", Count: 4},
		{Name: "Sun-Dappled Celebrant", Count: 3}, {Name: "Merrow Skyswimmer", Count: 3}, {Name: "Gallant Fowlknight", Count: 3},
		{Name: "Deepchannel Duelist", Count: 3}, {Name: "Adept Watershaper", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Eclipsed Boggart", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Heirloom Auntie", Count: 3},
		{Name: "Boggart Mischief", Count: 3}, {Name: "Flamekin Gildweaver", Count: 3}, {Name: "Scuzzback Scrounger", Count: 3},
		{Name: "Sizzling Changeling", Count: 3}, {Name: "Sourbread Auntie", Count: 3}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 6},
	},
}, {
	{
		{Name: "Eclipsed Merrow", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Wild Unraveling", Count: 4},
		{Name: "Sun-Dappled Celebrant", Count: 3}, {Name: "Merrow Skyswimmer", Count: 3}, {Name: "Gallant Fowlknight", Count: 3},
		{Name: "Deepchannel Duelist", Count: 3}, {Name: "Adept Watershaper", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Eclipsed Merrow", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Wild Unraveling", Count: 4},
		{Name: "Sun-Dappled Celebrant", Count: 3}, {Name: "Merrow Skyswimmer", Count: 3}, {Name: "Gallant Fowlknight", Count: 3},
		{Name: "Deepchannel Duelist", Count: 3}, {Name: "Adept Watershaper", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 5},
	},
}}

// castWinTapRun plays seeds 1..n of every matchup and returns the divergent games.
func castWinTapRun(t *testing.T, n uint64, report bool) (div int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	cfg := Config{}
	for mi, md := range castWinTapDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			m, err := NewMatchup(reg, [2]string{"cw-a", "cw-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if d := res.Div; d != nil {
				div++
				if !report {
					return div
				}
				t.Logf("last actions %v", actionStrings(res.Actions[max(d.At-8, 0):d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v\ngorge %v\ndirac %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

// TestCastWindowTapAnotherSourceLockstep: no seed diverges. The decks' convoke spells (Eclipsed
// Boggart's matchup, Merrow Skyswimmer) also pin that a creature announced for convoke is not
// offered to a Drum as the permanent to tap: gorge excludes it (pay.CastCommitted), dirac must.
func TestCastWindowTapAnotherSourceLockstep(t *testing.T) {
	t.Parallel()
	castWinTapRun(t, 150, true)
}

// TestCastWindowTapAnotherSourceMutation: with the cast window leaving the Drum out, some
// seed diverges, so the lockstep above reaches the Drum in the window.
func TestCastWindowTapAnotherSourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCastWinTap = true
	n := castWinTapRun(t, 150, false)
	dirac.DebugNoCastWinTap = false
	if n == 0 {
		t.Error("no seed diverged without the Drum in the cast window: the decks never reach it")
	}
}

// The recorded games: the rows that exposed the window, replayed in lockstep.
func TestCastWindowTapAnotherSourceRowsLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, name := range []string{
		"cast_window_tap_another_hybrid_merrow", "cast_window_tap_another_hybrid_boggart",
		"cast_window_tap_another_wild_unraveling", "cast_window_tap_another_wild_unraveling_either_or",
	} {
		t.Run(name, func(t *testing.T) {
			replayRepro(t, reg, cardsettest.Synth(t, "std-ecl"), name, false, false)
		})
	}
}
