package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Dragonhawk, Fate's Tempest (dirac/value_delayedexile.go, gorge's
// Count$ValidExile Card.IsTriggerRemembered): the dig on entering and on
// attacking registers its whole exiled set on a delayed end-step trigger,
// which deals 2 damage to each opponent per card still exiled. The pool plays
// the cards from exile (lands, burn, creatures, red Adventures) so the count
// shrinks, and ramps Dragonhawk out. The expected answer at every decision is
// gorge's.
const dragonhawkPool = "dragonhawkset"

func dragonhawkDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Dragonhawk, Fate's Tempest", Count: 6}, {Name: "Hearth Elemental", Count: 3}, {Name: "Bellowing Bruiser", Count: 3},
		{Name: "Craw Wurm", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5},
	}
	return [2][]cardset.Entry{d, d}
}

func TestDragonhawkLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, dragonhawkPool, dragonhawkDecks(), 600, "KDRAGONHAWK_SEEDS")
}

// TestDragonhawkLockstepMutation: with the count reading nothing, some game
// diverges (a Dragonhawk dug cards and the end step dealt damage), so the pool
// reaches the delayed trigger.
func TestDragonhawkLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDelayedExileOff = true
	defer func() { dirac.DebugDelayedExileOff = false }()
	if !lockstepDiverges(t, dragonhawkPool, dragonhawkDecks(), 600) {
		t.Fatal("no seed diverged with the delayed exile count at 0: no Dragonhawk end step dealt damage")
	}
}

// TestDragonhawkLockstepLooseMutation: with every remembered card counted
// whether or not it is still in exile, some game diverges (a dug card was
// played or cast before the end step).
func TestDragonhawkLockstepLooseMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDelayedExileLoose = true
	defer func() { dirac.DebugDelayedExileLoose = false }()
	if !lockstepDiverges(t, dragonhawkPool, dragonhawkDecks(), 600) {
		t.Fatal("no seed diverged counting played cards: no dug card left exile before the end step")
	}
}
