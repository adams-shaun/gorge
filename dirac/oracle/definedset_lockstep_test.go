package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Defined$ set-expression lockstep (kgen lower_definedset.go): Bounty
// Board's dies trigger reads Player.OpponentOf TriggeredCardController
// (SetTriggerPlayer SetPlayersComplement: each opponent of the dead
// creature's controller gains 2 life and draws). The pool is
// dirac/cardset/pools/fixtures/definedset; the kernel's SetStats count shows
// the games read a set.
const definedSetPool = "definedset"

var definedSetDecks = [2][]cardset.Entry{
	{
		{Name: "Bounty Board", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Glory Seeker", Count: 4}, {Name: "Screams from Within", Count: 4},
		{Name: "Swamp", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Bounty Board", Count: 4}, {Name: "Glory Seeker", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Screams from Within", Count: 4},
		{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 5},
	},
}

// definedSetGames plays the seeds and returns the clean games, the divergent
// ones and the kernel's set counts.
func definedSetGames(t *testing.T, hi uint64, mode lockstepMode, mutate func(*kop.Table)) (clean int, divs []lockstepDivergence, st *dirac.SetStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, definedSetPool, definedSetDecks)
	st = &dirac.SetStats{}
	dirac.DebugSetStats = st
	defer func() { dirac.DebugSetStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, definedSetPool, definedSetDecks, gd, 1, hi, mode, mutate, nil, castFirstPolicy)
	return c, d, st
}

// TestDefinedSetLockstep: no divergence and no kernel stall over 64 seeds,
// and across them a RefSet instruction ran.
func TestDefinedSetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	clean, divs, st := definedSetGames(t, 64, lockstepFailEach, nil)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Runs == 0 {
		t.Error("no game ran a RefSet instruction")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDefinedSetMutation: with every players complement read as every player
// (each opponent of the dead creature's controller becomes everyone), some
// game diverges from gorge.
func TestDefinedSetMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	rewritten := 0
	_, divs, st := definedSetGames(t, 64, lockstepQuiet, func(tbl *kop.Table) {
		for i, e := range tbl.Sets {
			if j := complementOverWhole(e); j >= 0 {
				// the complement and the operand it pops become one base,
				// so the program stays well formed
				tbl.Sets[i] = append(kop.SetExpr{{Kind: kop.SetEachPlayer}}, e[j+1:]...)
				rewritten++
			}
		}
	})
	if rewritten == 0 {
		t.Fatal("no set holds a SetPlayersComplement: the mutation changed nothing")
	}
	if st.Runs == 0 {
		t.Fatal("no game ran a RefSet instruction")
	}
	if len(divs) == 0 {
		t.Fatal("complement read as every player: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// complementOverWhole is the index of e's first SetPlayersComplement whose
// operand is everything before it (one set on the stack), or -1.
func complementOverWhole(e kop.SetExpr) int {
	d := 0
	for j, op := range e {
		if op.Kind == kop.SetPlayersComplement && d == 1 {
			return j
		}
		d += 1 - op.Kind.SetArity()
	}
	return -1
}
