package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Hermitic Herbalist's second mana ability makes two mana in any combination
// of colours, spent only on a Lesson spell: gorge's planner misses it, so the
// cast is the prefix search's (the ability ask, then the first ability's
// colour ask or the allocation's per-unit pairs, the play's colours first).
// The first deck's only blue source is the Herbalist, so a blue Lesson is
// cast only through the allocation answer that names blue, and Badgermole
// Cub's extra {G} rides every tap of the creature; the second deck has no
// Lesson (the restricted ability is never usable); the third holds White
// Lotus Hideout beside the Herbalist. Seed parity picks the surface.
func TestHermiticHerbalistComboLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	opp := []cardset.Entry{{Name: "Platypus-Bear", Count: 10}, {Name: "Ostrich-Horse", Count: 10}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10}}
	lessons := []cardset.Entry{{Name: "Hermitic Herbalist", Count: 5}, {Name: "Badgermole Cub", Count: 4}, {Name: "Sokka's Haiku", Count: 3},
		{Name: "Waterbending Lesson", Count: 4}, {Name: "It'll Quench Ya!", Count: 2}, {Name: "Rockalanche", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 9}}
	noLesson := []cardset.Entry{{Name: "Hermitic Herbalist", Count: 6}, {Name: "Badgermole Cub", Count: 4}, {Name: "Platypus-Bear", Count: 6},
		{Name: "Ostrich-Horse", Count: 6}, {Name: "Island", Count: 9}, {Name: "Forest", Count: 9}}
	hideout := []cardset.Entry{{Name: "Hermitic Herbalist", Count: 4}, {Name: "White Lotus Hideout", Count: 4}, {Name: "Waterbending Lesson", Count: 4},
		{Name: "Sokka's Haiku", Count: 3}, {Name: "True Ancestry", Count: 3}, {Name: "Rockalanche", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 9}}
	// 150 seeds each; gorge's prefix searches make the Hideout matchup the
	// slow one, so its seeds run as two halves beside the other two
	for _, m := range []struct {
		name        string
		deck        []cardset.Entry
		first, last int
	}{{"lessons", lessons, 1, 150}, {"no-lesson", noLesson, 1, 150}, {"hideout-1", hideout, 1, 75}, {"hideout-2", hideout, 76, 150}} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			fixtureLockstepSeeds(t, "std-tla", [2][]cardset.Entry{m.deck, opp}, m.first, m.last)
		})
	}
}
