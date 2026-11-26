package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The graveyard ability-loss lockstep (CR 613.1f): Yixlid Jailer's
// S:Mode$ Continuous | Affected$ Card | AffectedZone$ Graveyard |
// RemoveAllAbilities$ True removes the printed abilities of every card in a
// graveyard, so its flashback, unearth, dredge, graveyard trigger and
// graveyard mana ability do not exist while the Jailer is out. The kernel
// hides each offer where gorge's printed-face gates do
// (dirac/static_graveabilityloss.go).
//
// The pool is the offzoneabilityloss fixture: the Jailer beside the
// graveyard abilities it silences, discard and self-mill filling the
// graveyards, and no graveyard keyword grantor (a grant beside the removal
// stalls by name, StallGraveLossGrant).
const offZoneLossPool = "offzoneabilityloss"

// offZoneLossPodPool is the four-seat pool: the same cards without Bloodghast
// and Jack-o'-Lantern.
const offZoneLossPodPool = "offzoneabilitylosspod"

// offZoneLossDeck is a 40-card deck: the Jailer, graveyard-ability cards and
// the fillers that put them into the graveyard.
func offZoneLossDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Yixlid Jailer", Count: 4}, {Name: "Faithless Looting", Count: 3}, {Name: "Think Twice", Count: 2},
		{Name: "Dregscape Zombie", Count: 3}, {Name: "Bloodghast", Count: 3}, {Name: "Golgari Thug", Count: 2},
		{Name: "Jack-o'-Lantern", Count: 2}, {Name: "Cathartic Reunion", Count: 2}, {Name: "Stitcher's Supplier", Count: 3},
		{Name: "Anger", Count: 2},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 2},
	}
}

func offZoneLossDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{offZoneLossDeck(), offZoneLossDeck()}
}

func TestOffZoneAbilityLossLockstep(t *testing.T) {
	before := dirac.GraveLossStats.Hits.Load()
	decks := offZoneLossDecks()
	_, set, gd := lockstepPool(t, offZoneLossPool, decks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("the deck is not playable in the offzoneabilityloss pool")
	}
	if n := lockstepRun(t, offZoneLossPool, decks, 64, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	if n := dirac.GraveLossStats.Hits.Load() - before; n == 0 {
		t.Fatal("the Jailer hid no graveyard ability; the lockstep proves nothing")
	} else {
		t.Logf("graveyard abilities hidden: %d", n)
	}
}

func TestFourSeatOffZoneAbilityLossLockstep(t *testing.T) {
	before := dirac.GraveLossStats.Hits.Load()
	// Bloodghast ("an opponent has 10 or less life" reads the one opponent)
	// and Jack-o'-Lantern (the graveyard mana window) are two-seat cards, so
	// the pod pool (offzoneabilitylosspod) leaves them out and the pod plays
	// Grizzly Bears in their place.
	var d []string
	for _, e := range offZoneLossDeck() {
		if e.Name == "Bloodghast" || e.Name == "Jack-o'-Lantern" {
			e.Name = "Grizzly Bears"
		}
		d = append(d, deckOf(e.Count, e.Name)...)
	}
	if n := pod4FixtureLockstep(t, offZoneLossPodPool, [4][]string{d, d, d, d}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	if dirac.GraveLossStats.Hits.Load()-before == 0 {
		t.Fatal("the Jailer hid no graveyard ability in any four-seat game")
	}
}

// TestOffZoneAbilityLossMutation proves the pool reaches the loss: with the
// predicate answering "not lost" some game diverges from gorge.
func TestOffZoneAbilityLossMutation(t *testing.T) {
	dirac.DebugNoGraveLoss = true
	defer func() { dirac.DebugNoGraveLoss = false }()
	if lockstepRun(t, offZoneLossPool, offZoneLossDecks(), 64, false, nil) == 0 {
		t.Fatal("DebugNoGraveLoss changed no game: the pool does not reach the graveyard ability loss")
	}
}
