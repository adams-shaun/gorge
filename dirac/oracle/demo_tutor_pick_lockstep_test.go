package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The tutor-pick lockstep: Fertilid ("target player searches their library
// for a basic land card, puts it onto the battlefield tapped": Chooser$
// Targeted with no DefinedPlayer$, so the targeted player's own library) and
// Intuition / Gifts Ungiven (search your library, then a targeted opponent
// chooses which of the remembered cards go where, the rest swept to the
// graveyard or hand and the library shuffled only when a swept card left it).
// The pool is dirac/cardset/pools/fixtures/demotutorpick. The kernel's
// TutorPickStats count proves an opponent answered a search, and the mutation
// clears that read so the searcher answers instead.
const demoTutorPickPool = "demotutorpick"

var demoTutorPickDecks = [2][]cardset.Entry{
	{
		{Name: "Intuition", Count: 4}, {Name: "Fertilid", Count: 4}, {Name: "Gifts Ungiven", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Sol Ring", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 2}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Gifts Ungiven", Count: 4}, {Name: "Intuition", Count: 3}, {Name: "Fertilid", Count: 4},
		{Name: "Glory Seeker", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Swamp", Count: 2}, {Name: "Plains", Count: 2},
	},
}

// demoTutorPickGames plays the seeds and returns the clean games, the
// divergent ones and the count of searches an opponent answered.
func demoTutorPickGames(t *testing.T, hi uint64, mode lockstepMode, mutate func(*kop.Table)) (clean int, divs []lockstepDivergence, st *dirac.TutorPickStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoTutorPickPool, demoTutorPickDecks)
	st = &dirac.TutorPickStats{}
	dirac.DebugTutorPickStats = st
	defer func() { dirac.DebugTutorPickStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoTutorPickPool, demoTutorPickDecks, gd, 1, hi, mode, mutate, nil, castFirstPolicy)
	return c, d, st
}

// TestDemoTutorPickLockstep: no divergence and no kernel stall over 64 seeds,
// and across them an opponent answered a library search.
func TestDemoTutorPickLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	clean, divs, st := demoTutorPickGames(t, 64, lockstepFailEach, nil)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.OppAnswered == 0 {
		t.Error("no game had an opponent answer a library search")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoTutorPickMutation: with every search's opponent-chooser bit cleared
// the searcher answers the pick, and some game diverges from gorge.
func TestDemoTutorPickMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	cleared := 0
	_, divs, _ := demoTutorPickGames(t, 64, lockstepQuiet, func(tbl *kop.Table) {
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				prog := tbl.Cards[ci].Abilities[ai].Prog
				for i := range prog {
					if prog[i].Op == kop.OpSearchEx && prog[i].B.N&kop.SrChooserOpp != 0 {
						prog[i].B.N &^= kop.SrChooserOpp
						cleared++
					}
				}
			}
		}
	})
	if cleared == 0 {
		t.Fatal("no search carries SrChooserOpp: the mutation changed nothing")
	}
	if len(divs) == 0 {
		t.Fatal("opponent chooser cleared: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestDemoTutorPickFertilidMutation: with every search's enters-tapped bit
// cleared (Fertilid's is the only one in the pool) some game diverges, so the
// pool does activate Fertilid's targeted-player search.
func TestDemoTutorPickFertilidMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	cleared := 0
	_, divs, _ := demoTutorPickGames(t, 64, lockstepQuiet, func(tbl *kop.Table) {
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				prog := tbl.Cards[ci].Abilities[ai].Prog
				for i := range prog {
					if prog[i].Op == kop.OpSearchEx && prog[i].C>>3&kop.SrTapped != 0 {
						prog[i].C &^= kop.SrTapped << 3
						cleared++
					}
				}
			}
		}
	})
	if cleared == 0 {
		t.Fatal("no search enters tapped: the mutation changed nothing")
	}
	if len(divs) == 0 {
		t.Fatal("enters-tapped cleared: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoTutorPickLockstep: four seats, gorge leading. Fertilid
// (Search obj=Target0, any seat count) plays four-seat games with no
// divergence and no kernel stop. Intuition and Gifts Ungiven are two-seat only
// (SrChooserOpp names the controller's first opponent, not the targeted one,
// which differ past two seats): their pod stalls on the declared StallTwoSeat
// limit, never diverging from gorge.
func TestFourSeatDemoTutorPickLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Fertilid"), deckOf(4, "Grizzly Bears"), deckOf(3, "Llanowar Elves"), deckOf(2, "Sol Ring"),
		deckOf(8, "Forest"), deckOf(4, "Island"), deckOf(2, "Plains"))
	b := pod4Deck(deckOf(4, "Fertilid"), deckOf(4, "Glory Seeker"), deckOf(3, "Ornithopter"), deckOf(2, "Llanowar Elves"),
		deckOf(8, "Forest"), deckOf(4, "Island"), deckOf(2, "Swamp"))
	if n := pod4FixtureLockstep(t, demoTutorPickPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	ta := pod4Deck(deckOf(4, "Intuition"), deckOf(4, "Fertilid"), deckOf(3, "Gifts Ungiven"),
		deckOf(4, "Grizzly Bears"), deckOf(3, "Llanowar Elves"), deckOf(8, "Island"), deckOf(8, "Forest"))
	tb := pod4Deck(deckOf(4, "Gifts Ungiven"), deckOf(3, "Intuition"), deckOf(4, "Fertilid"),
		deckOf(4, "Glory Seeker"), deckOf(3, "Ornithopter"), deckOf(8, "Island"), deckOf(8, "Forest"))
	if n := pod4FixtureLockstep(t, demoTutorPickPool, [4][]string{ta, tb, ta, tb}, 30, false, nil); n != 0 {
		t.Fatalf("%d diverged with the two-seat tutors", n)
	}
}
