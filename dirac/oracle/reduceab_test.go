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

// kreduceabManaDecks put a floored activated-ability reducer (Agatha of the
// Vile Cauldron, "can't reduce the mana in that cost to less than one mana")
// beside mana abilities that cost one mana: filter lands and creatures
// (Crystal Grotto, Rootrider Faun, Scarecrow Guide). The floor leaves each
// as printed, which is what gorge plays.
var kreduceabManaDecks = [2][]cardset.Entry{
	{
		{Name: "Agatha of the Vile Cauldron", Count: 4}, {Name: "Rootrider Faun", Count: 4},
		{Name: "Scarecrow Guide", Count: 4}, {Name: "Knowledge Seeker", Count: 4},
		{Name: "Crystal Grotto", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 4},
		{Name: "Mutavault", Count: 2}, {Name: "Treetop Village", Count: 4},
	},
	{
		{Name: "Agatha of the Vile Cauldron", Count: 4}, {Name: "Scarecrow Guide", Count: 4},
		{Name: "Boom Scholar", Count: 3}, {Name: "Greasewrench Goblin", Count: 3},
		{Name: "Camera Launcher", Count: 3}, {Name: "Crystal Grotto", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mutavault", Count: 2},
		{Name: "Rogue's Passage", Count: 1},
	},
}

// TestReduceabManaCostedLockstep: a StatReduceAbility static beside a mana
// ability with a mana cost it leaves as printed, on both surfaces.
func TestReduceabManaCostedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "reduceab", kreduceabManaDecks, 60, "KREDUCEAB_SEEDS")
}

// kreduceabFilterDecks put Blossoming Tortoise ("activated abilities of lands
// you control cost {1} less") beside Crystal Grotto, whose "{1}, {T}: Add one
// mana of any color" it turns into a free any-colour tap, so a filter land
// makes the second colour with nothing else tapped (CR 601.2f, 602.2b; gorge
// manaAbilityCC). The spells ask for the colours the Grotto must make.
var kreduceabFilterDecks = [2][]cardset.Entry{
	{
		{Name: "Blossoming Tortoise", Count: 4}, {Name: "Crystal Grotto", Count: 8},
		{Name: "Knowledge Seeker", Count: 4}, {Name: "Forecasting Fortune Teller", Count: 4},
		{Name: "Keen Buccaneer", Count: 4}, {Name: "Greenbelt Guardian", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Crystal Grotto", Count: 8}, {Name: "Agatha of the Vile Cauldron", Count: 4},
		{Name: "Boom Scholar", Count: 4}, {Name: "Greasewrench Goblin", Count: 4},
		{Name: "Camera Launcher", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// TestReduceabFilterLockstep: a land discount beside filter lands, on both
// surfaces.
func TestReduceabFilterLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "reduceab", kreduceabFilterDecks, 60, "KREDUCEAB_SEEDS")
}

// TestReduceabFilterMutation proves the pool plays a discounted filter: with
// the reduction dropped, some seed diverges.
func TestReduceabFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAbReduce = true
	defer func() { dirac.DebugNoAbReduce = false }()
	if !lockstepDiverges(t, "reduceab", kreduceabFilterDecks, 200) {
		t.Fatal("no seed diverged with the filter's reduction dropped: the pool never plays a discounted filter")
	}
}
