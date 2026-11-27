package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The RepeatEach ChangeZoneTable$ Token/Copy lockstep (Ezuri's Predation and
// Mirror-Style Master): Ezuri's ordered loop creates a Beast per opponent
// creature and each Beast fights its creature; Mirror's batched CopyPermanent
// copies each attacking modified creature and the DpMany end-of-combat trigger
// exiles the copies. The pool is dirac/cardset/pools/fixtures/repeateachczt.
// RepeatCztStats proves the shape resolved (tokens, copies, fights, exiles and
// the order ask), so a green game is not one where the loop never ran.
const repeateachcztPool = "repeateachczt"

var repeateachcztDecks = [2][]cardset.Entry{
	{
		{Name: "Ezuri's Predation", Count: 3}, {Name: "Grizzly Bears", Count: 8}, {Name: "Glory Seeker", Count: 5},
		{Name: "Forest", Count: 24},
	},
	{
		{Name: "Mirror-Style Master", Count: 3}, {Name: "Grizzly Bears", Count: 8}, {Name: "Bonesplitter", Count: 3},
		{Name: "Rancor", Count: 2}, {Name: "Mountain", Count: 24},
	},
}

// repeateachcztGames plays the seeds and returns the clean games, the
// divergent ones and the shape counters.
func repeateachcztGames(t *testing.T, hi uint64, mode lockstepMode) (clean, divs int, st *dirac.RepeatCztStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, repeateachcztPool, repeateachcztDecks)
	st = &dirac.RepeatCztStats{}
	dirac.DebugRepeatCztStats = st
	defer func() { dirac.DebugRepeatCztStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, repeateachcztPool, repeateachcztDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, len(d), st
}

// TestDemoRepeateachCztLockstep: no divergence and no kernel stall over 64
// seeds, and across them both shapes resolved.
func TestDemoRepeateachCztLockstep(t *testing.T) {
	clean, divs, st := repeateachcztGames(t, 64, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	if st.Tokens == 0 || st.Fights == 0 || st.Copies == 0 || st.Exiles == 0 || st.Ordered == 0 {
		t.Errorf("shape counters %+v: want a token, a fight, a copy, an exile and an order ask", *st)
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoRepeateachCztMutation: with the batched body disabled, and again
// with only the DpMany end-of-combat exile disabled, some game diverges from
// gorge. The narrow switch pins the new reads, not just the loop body: the
// copies must actually leave at end of combat.
func TestDemoRepeateachCztMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRepeatEachCztOff = true
	defer func() { dirac.DebugRepeatEachCztOff = false }()
	_, divs, _ := repeateachcztGames(t, 64, lockstepQuiet)
	dirac.DebugRepeatEachCztOff = false
	if divs == 0 {
		t.Fatal("DebugRepeatEachCztOff: no divergence in 64 games")
	}
	t.Logf("whole-loop mutation diverged in %d games", divs)
	dirac.DebugRepeatEachCztExileOff = true
	defer func() { dirac.DebugRepeatEachCztExileOff = false }()
	_, divs, _ = repeateachcztGames(t, 64, lockstepQuiet)
	dirac.DebugRepeatEachCztExileOff = false
	if divs == 0 {
		t.Fatal("DebugRepeatEachCztExileOff: no divergence in 64 games")
	}
	t.Logf("exile-only mutation diverged in %d games", divs)
}

// TestFourSeatDemoRepeateachCztLockstep: four seats, gorge leading, Ezuri's
// Predation (its order ask and batch hold at four seats). Mirror-Style
// Master is not played four-seat: its batched copy enters attacking
// nextSeat, not gorge's CR 508.4 defender ask, so the card is marked
// two-seat and a pod refuses its table at the start (StallTwoSeat) rather
// than desyncing silently — asserted here.
func TestFourSeatDemoRepeateachCztLockstep(t *testing.T) {
	a := pod4Deck(deckOf(3, "Ezuri's Predation"), deckOf(8, "Grizzly Bears"), deckOf(5, "Glory Seeker"), deckOf(24, "Forest"))
	if n := pod4FixtureLockstep(t, repeateachcztPool, [4][]string{a, a, a, a}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	set := cardsettest.Set(t, repeateachcztPool)
	i, ok := set.Lookup("Mirror-Style Master")
	if !ok {
		t.Fatal("Mirror-Style Master is not in the repeateachczt pool")
	}
	if d := set.Def(i); !kop.CardTwoSeat(d) {
		t.Error("Mirror-Style Master is not marked two-seat: a pod would attack the next seat silently")
	}
}
