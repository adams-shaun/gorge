package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: the "if you cast a spell this way, it enters with a
// counter" carriers — Intrepid Paleontologist, Leonardo Sewer Samurai,
// Osteomancer Adept and Overflow Submatrix (a FINALITY counter), Undead
// Sprinter and Mikey & Don, Party Planners (an extra +1/+1 counter) — whose
// Static$ True SpellCast triggers resolve off the stack, before the
// CR 603.3b ordering ask, and whose bodies only stamp the cast spell's
// stack entry. The may-play-source provenance reads the cast's flag, so an
// ordinary cast of the same cards triggers nothing (the OrdinaryCasts
// count). Counterspell and Unsummon put carriers' spells back where they
// can be cast again: the rider's Effect ends once its spell leaves the
// stack in both engines (CR 400.7), so a countered spell recast from the
// graveyard gets only its new cast's counter and a bounced permanent recast
// from hand gets none.
var mayPlayDecks = [4][]cardset.Entry{
	{ // B/G: the three FINALITY carriers, Undead Sprinter, dinosaur and
		// 1-power fodder, self-mill, removal and basics
		{Name: "Intrepid Paleontologist", Count: 3}, {Name: "Leonardo, Sewer Samurai", Count: 3}, {Name: "Osteomancer Adept", Count: 4},
		{Name: "Undead Sprinter", Count: 4}, {Name: "Stitcher's Supplier", Count: 4},
		{Name: "Colossal Dreadmaw", Count: 3}, {Name: "Thrashing Brontodon", Count: 2}, {Name: "Typhoid Rats", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Murder", Count: 2}, {Name: "Return to Nature", Count: 2},
		{Name: "Forest", Count: 4}, {Name: "Swamp", Count: 4},
	},
	{ // G/U: Mikey & Don and the Mutants/Ninjas/Turtles it may cast from the
		// library top (changelings count), bounce removal and basics
		{Name: "Mikey & Don, Party Planners", Count: 4}, {Name: "Woodland Changeling", Count: 4},
		{Name: "Game-Trail Changeling", Count: 2}, {Name: "Horned Turtle", Count: 2},
		{Name: "Ninja of the Deep Hours", Count: 2}, {Name: "Mothdust Changeling", Count: 2},
		{Name: "Unsummon", Count: 3}, {Name: "Naturalize", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Forest", Count: 7}, {Name: "Island", Count: 8},
	},
	{ // the green-blue deck's mirror is the {1, 1} matchup; the dense
		// carrier deck is deck 0 against itself (Leonardo beside an active
		// Osteomancer effect covers one cast with two FINALITY grants). Four
		// Osteomancers at most: each activation is an until-end-of-turn
		// emblem, and maxEmb is 4 per player (StallEmblem past it)
		{Name: "Intrepid Paleontologist", Count: 4}, {Name: "Leonardo, Sewer Samurai", Count: 4}, {Name: "Osteomancer Adept", Count: 4},
		{Name: "Undead Sprinter", Count: 6}, {Name: "Stitcher's Supplier", Count: 4},
		{Name: "Colossal Dreadmaw", Count: 4}, {Name: "Typhoid Rats", Count: 6},
		{Name: "Murder", Count: 2}, {Name: "Forest", Count: 3}, {Name: "Swamp", Count: 3},
	},
	{ // U/B: Overflow Submatrix and the artifact creatures it casts from the
		// graveyard, self-mill, Undead Sprinter (Overflow's flag-only
		// may-play read covers its graveyard cast too), Counterspell and
		// Unsummon for the recasts, removal and basics
		{Name: "Overflow Submatrix", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Memnite", Count: 3},
		{Name: "Bronze Sable", Count: 3}, {Name: "Phyrexian Walker", Count: 2}, {Name: "Yotian Soldier", Count: 2},
		{Name: "Stitcher's Supplier", Count: 3}, {Name: "Undead Sprinter", Count: 2}, {Name: "Counterspell", Count: 3},
		{Name: "Unsummon", Count: 2}, {Name: "Murder", Count: 2}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 6},
	},
}

// mayPlayMatchups: A vs B (both players' casts carry the other's riders;
// B's Counterspell and Unsummon send A's carriers back to be recast), the
// dense carrier mirror (two FINALITY grants on one cast: Leonardo beside an
// active Osteomancer effect), B vs B (the P1P1 top-library casts), and the
// Overflow Submatrix deck against B (countered and bounced artifact
// creatures recast) and against A (each side's graveyard casts beside an
// Overflow Submatrix).
var mayPlayMatchups = [][2]int{{0, 1}, {2, 2}, {1, 1}, {3, 1}, {3, 0}}

func mayPlayRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mayplaystatic")
	if err != nil {
		t.Fatal(err)
	}
	for _, mu := range mayPlayMatchups {
		var decks [2][]*cards.Card
		for s, di := range mu {
			for _, e := range mayPlayDecks[di] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("deck %d: %d cards", di, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(mayPlayDecks[mu[0]], mayPlayDecks[mu[1]])
		if err != nil {
			t.Fatal(err)
		}
		if d, st := mayPlaySeeds(t, reg, set, tbl, decks, seeds, report); d {
			return true, stalls + st
		} else {
			stalls += st
		}
	}
	return false, stalls
}

func mayPlaySeeds(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *dirac.Table, decks [2][]*cards.Card, seeds uint64, report bool) (diverged bool, stalls int) {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kmayplaystatic-a", "kmayplaystatic-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := Policy(RandomPolicy(seed))
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return true, stalls
		}
	}
	return false, stalls
}

func TestMayplaystaticLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.MayPlayStats.StaticResolved.Store(0)
	dirac.MayPlayStats.FinalityEnter.Store(0)
	dirac.MayPlayStats.OrdinaryCasts.Store(0)
	dirac.EffEtbStats.Applied.Store(0)
	div, stalls := mayPlayRun(t, 256, true)
	static, finality, ordinary, p1p1 := dirac.MayPlayStats.StaticResolved.Load(), dirac.MayPlayStats.FinalityEnter.Load(), dirac.MayPlayStats.OrdinaryCasts.Load(), dirac.EffEtbStats.Applied.Load()
	t.Logf("256 seeds, %d divergences, %d stalls; static resolutions %d, FINALITY entries %d, +1/+1 applications %d, ordinary casts of the carriers %d", b2i(div), stalls, static, finality, p1p1, ordinary)
	if div {
		return
	}
	for _, c := range []struct {
		s        string
		got, min int64
	}{
		{"static-trigger resolutions", static, 30},
		{"permanents entering with a FINALITY counter", finality, 20},
		{"permanents entering with the extra +1/+1 counter", p1p1, 20},
		{"ordinary casts of the carriers that do not trigger", ordinary, 10},
	} {
		if c.got < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.got, c.min)
		}
	}
}

// TestMayplaystaticMutation: every cast reading as a may-play cast must
// make the lockstep diverge (the riders land on ordinary casts too).
func TestMayplaystaticMutation(t *testing.T) {
	dirac.DebugMayPlaySrcAny = true
	defer func() { dirac.DebugMayPlaySrcAny = false }()
	if diverged, _ := mayPlayRun(t, 64, false); !diverged {
		t.Fatal("DebugMayPlaySrcAny: no divergence in 64 seeds")
	}
}
