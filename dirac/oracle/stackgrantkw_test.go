package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The AffectedZone$ Stack cost-keyword grants: "Creature spells you cast have
// convoke" (Dazzling Theater // Prop Room, Eirdu, Carrier of Dawn; CR
// 702.51a). gorge reads the granted keyword under the stack-zone override at
// every convoke reader (rules cast_altcost.go hasCastConvoke), so the spells
// the controller casts announce tapped creatures like a printed convoke card.
// Those two cards stay refused for another class (Prop Room's
// UntapOtherPlayer, Isilu's Persist grant), so the pool plays the same static
// from Chief Engineer (artifact spells) and We Ride at Dawn (legendary
// creature spells). The expected answer at every decision is gorge's.
var kstackgrantConvokeDecks = [2][]cardset.Entry{
	{
		{Name: "Chief Engineer", Count: 4}, {Name: "We Ride at Dawn", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Pilgrim's Eye", Count: 3}, {Name: "Mind Stone", Count: 2}, {Name: "Sol Ring", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Divination", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 2},
	},
}

func TestStackGrantKeywordLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stackgrantkw", kstackgrantConvokeDecks, 300, "KSTACKGRANTKW_SEEDS")
}

// TestStackGrantKeywordMutation: with the kernel ignoring the grants some
// game diverges, so the pool does reach a granted convoke cast.
func TestStackGrantKeywordMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugStackKwOff = true
	defer func() { dirac.DebugStackKwOff = false }()
	if !lockstepDiverges(t, "stackgrantkw", kstackgrantConvokeDecks, 120) {
		t.Fatal("no game diverged with the grants ignored: the pool never casts a creature spell with a granted convoke")
	}
}

// kstackgrantMixedDecks: Chief Engineer grants Convoke to Kappa Cannoneer,
// which has printed Improvise: gorge's one announcement offers creature
// colour, creature generic and artifact generic picks (mirrored in both seats).
var kstackgrantMixedDecks = mirroredDeck([]cardset.Entry{
	{Name: "Chief Engineer", Count: 4}, {Name: "Kappa Cannoneer", Count: 4}, {Name: "Pilgrim's Eye", Count: 4},
	{Name: "Mind Stone", Count: 4}, {Name: "Island", Count: 24},
})

// kstackgrantXDecks: the granted Convoke on an {X} artifact spell (Walking
// Ballista): gorge announces before X and offers every creature a possible
// generic payment, then prices the picks into the X menu.
var kstackgrantXDecks = mirroredDeck([]cardset.Entry{
	{Name: "Chief Engineer", Count: 4}, {Name: "Walking Ballista", Count: 4}, {Name: "Pilgrim's Eye", Count: 4},
	{Name: "Mind Stone", Count: 4}, {Name: "Island", Count: 24},
})

func mirroredDeck(d []cardset.Entry) [2][]cardset.Entry { return [2][]cardset.Entry{d, d} }

func TestStackGrantMixedKeywordsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stackgrantkw", kstackgrantMixedDecks, 24, "KSTACKGRANTMIXED_SEEDS")
}

func TestStackGrantXCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stackgrantkw", kstackgrantXDecks, 16, "KSTACKGRANTX_SEEDS")
}
