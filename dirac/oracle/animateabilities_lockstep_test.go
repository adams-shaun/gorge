package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Animate Abilities$ granted to a permanent other than the source (kgen
// animateSelfAbilities): Banishing Knack and Retraction Helix give a creature
// "{T}: return target nonland permanent", Run Wild, Defiling Tears and
// Greater Stone Spirit a regeneration or pump, Touch of Vitae an untap,
// Evolution Vat a counter doubler, Genju of the Fens the enchanted Swamp its
// pump. Gorge offers the granted activation against the object of the card
// that made the effect (CR 114.4), so the engines agree decision by decision
// only when the kernel's record keeps that maker's object.
func animateAbilitiesDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Banishing Knack", Count: 4}, {Name: "Retraction Helix", Count: 3}, {Name: "Run Wild", Count: 3},
		{Name: "Evolution Vat", Count: 3}, {Name: "Touch of Vitae", Count: 3}, {Name: "Greater Stone Spirit", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5},
	}
	b := []cardset.Entry{
		{Name: "Defiling Tears", Count: 4}, {Name: "Genju of the Fens", Count: 4}, {Name: "Glorious Sunrise", Count: 3},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Raging Goblin", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	}
	return [2][]cardset.Entry{a, b}
}

func TestAnimateAbilitiesLowers(t *testing.T) {
	t.Parallel()
	lockstepRunPolicy(t, "animateabilities", animateAbilitiesDecks(), 200, true, nil, nil, animateNoRepeatPolicy)
}
