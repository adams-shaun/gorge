package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The AnimateAll parameter lockstep: RemoveKeywords$
// (Spectacular Pileup, The Fire Nation Drill, Quick Draw), RemoveAllAbilities$
// (Curious Colossus, Final Showdown) and the target-player scope
// (ValidTgts$ Opponent: Quick Draw, Curious Colossus), gorge effAnimateAll.
// Deck A is the sweepers and their targets; deck B the keyword creatures
// they meet.
var kanimallDecks = [2][]cardset.Entry{
	{
		{Name: "Spectacular Pileup", Count: 3}, {Name: "Quick Draw", Count: 4},
		{Name: "Final Showdown", Count: 3}, {Name: "Curious Colossus", Count: 2},
		{Name: "Youthful Knight", Count: 4}, {Name: "Fencing Ace", Count: 3},
		{Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "The Fire Nation Drill", Count: 3}, {Name: "Darksteel Myr", Count: 4},
		{Name: "Gladecover Scout", Count: 4}, {Name: "Elite Vanguard", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestAnimallLockstep(t *testing.T) {
	t.Parallel()
	sc, rk, la := dirac.AnimAllScoped.Load(), dirac.AnimAllRemoveKw.Load(), dirac.AnimAllLoseAb.Load()
	lockstepSet(t, "animatevalue", kanimallDecks, 128, "KANIMALL_SEEDS")
	sc, rk, la = dirac.AnimAllScoped.Load()-sc, dirac.AnimAllRemoveKw.Load()-rk, dirac.AnimAllLoseAb.Load()-la
	t.Logf("%d scoped OverrideAll resolutions, %d RemoveKeywords$ hits, %d RemoveAllAbilities$ hits", sc, rk, la)
	// 128 seeds meet a keyword-holding permanent with a RemoveKeywords$ grant
	// only 4 times (the keyword creatures rarely share the battlefield with
	// the three one-shot sweepers), so the floor is 3 rather than 5.
	if sc < 10 || rk < 3 || la < 3 {
		t.Fatalf("pool under-exercised: scoped %d (<10), remove-keyword hits %d (<3), lose-abilities hits %d (<3)", sc, rk, la)
	}
}

// TestAnimallMutation proves the pool resolves a scoped AnimateAll: with
// the scope ignored (both seats swept) some seed diverges.
func TestAnimallMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugAnimAllNoScope = true
	defer func() { dirac.DebugAnimAllNoScope = false }()
	if !lockstepDiverges(t, "animatevalue", kanimallDecks, 64) {
		t.Fatal("no seed diverged with the AnimateAll scope ignored: the pool never resolves a scoped sweep")
	}
}
