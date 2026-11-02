package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The activated-ability cost-reduction lockstep:
// "activated abilities of <permanents you control> cost {N} less" (Mutagen
// Man for artifact tokens, Blossoming Tortoise for lands, Boom Scholar for
// the other Exhaust abilities, CR 601.2f / 602.2b). Deck A holds Clue makers
// and utility lands under the first two, deck B Exhaust creatures under the
// third; the expected answer at every decision (the offer, the autopay plan,
// the charge) is gorge's.
var kreduceabDecks = [2][]cardset.Entry{
	{
		{Name: "Mutagen Man, Living Ooze", Count: 3}, {Name: "Blossoming Tortoise", Count: 4},
		{Name: "Knowledge Seeker", Count: 4}, {Name: "Forecasting Fortune Teller", Count: 3},
		{Name: "Keen Buccaneer", Count: 3}, {Name: "Greenbelt Guardian", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 7},
		{Name: "Mutavault", Count: 1}, {Name: "Treetop Village", Count: 1},
		{Name: "Faerie Conclave", Count: 1}, {Name: "Stirring Wildwood", Count: 1},
		{Name: "Blighted Woodland", Count: 1}, {Name: "Mishra's Foundry", Count: 1},
	},
	{
		{Name: "Boom Scholar", Count: 4}, {Name: "Greasewrench Goblin", Count: 3},
		{Name: "Pacesetter Paragon", Count: 3}, {Name: "Camera Launcher", Count: 3},
		{Name: "Stampeding Scurryfoot", Count: 3}, {Name: "Fire Nation Raider", Count: 2},
		{Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		{Name: "Rogue's Passage", Count: 1}, {Name: "Mutavault", Count: 1},
		{Name: "Treetop Village", Count: 1}, {Name: "Slayers' Stronghold", Count: 1},
	},
}

// 60 seeds fit the 2-minute test cap (about a second a game); KREDUCEAB_SEEDS=1000
// is the full sweep.
func TestReduceabLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.AbReducedPriced.Load()
	lockstepSet(t, "reduceab", kreduceabDecks, 60, "KREDUCEAB_SEEDS")
	n := dirac.AbReducedPriced.Load() - before
	t.Logf("%d ability pricings lowered by a StatReduceAbility static", n)
	if n < 40 {
		t.Fatalf("only %d reduced ability pricings: the pool does not exercise the static", n)
	}
}

// TestReduceabMutation proves the pool activates a reduced ability: with
// the reduction dropped, some seed diverges.
func TestReduceabMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAbReduce = true
	defer func() { dirac.DebugNoAbReduce = false }()
	if !lockstepDiverges(t, "reduceab", kreduceabDecks, 200) {
		t.Fatal("no seed diverged with the ability reduction dropped: the pool never activates a reduced ability")
	}
}
