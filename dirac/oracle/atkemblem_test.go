package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Triggers an Effect delivers to attacks, lasting this turn or until its
// controller's next turn: the Last Ronin's chapter III ("whenever a creature
// you control attacks alone this turn ..."), Garruk, Curse Breaker's [-4]
// ("whenever one or more creatures attack one of your opponents ...") and
// Jace, Reality Sculptor's [-3] ("whenever a creature attacks you or a
// planeswalker you control ..."). The emblem trigger is queued once per
// attacker, or once per defender's group, after the declaration's own triggers
// (CR 508.1, 603.2).
var atkEmblemDecks = [3][]cardset.Entry{
	{
		{Name: "The Last Ronin", Count: 4}, {Name: "Garruk, Curse Breaker", Count: 4},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Giant Growth", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Jace, Reality Sculptor", Count: 4}, {Name: "Wind Drake", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Garruk, Curse Breaker", Count: 3}, {Name: "Jace, Reality Sculptor", Count: 3}, {Name: "The Last Ronin", Count: 2},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Wind Drake", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 4},
	},
}

func TestAttackEmblemLockstep(t *testing.T) {
	t.Parallel()
	d := atkEmblemDecks
	lockstepSet(t, "atkemblem", [2][]cardset.Entry{d[0], d[1]}, 64, "KATKEMBLEM_SEEDS")
	lockstepSet(t, "atkemblem", [2][]cardset.Entry{d[1], d[2]}, 64, "KATKEMBLEM_SEEDS")
	lockstepSet(t, "atkemblem", [2][]cardset.Entry{d[2], d[0]}, 64, "KATKEMBLEM_SEEDS")
}

// TestAttackEmblemMutation proves each emblem reaches the lockstep: with the
// hook off, a deck holding only one of the three planeswalkers and the saga
// diverges.
func TestAttackEmblemMutation(t *testing.T) {
	dirac.DebugIgnoreAtkEmblem = true
	defer func() { dirac.DebugIgnoreAtkEmblem = false }()
	foe := atkEmblemDecks[1]
	for _, c := range []struct {
		name string
		deck []cardset.Entry
	}{
		{"The Last Ronin", []cardset.Entry{
			{Name: "The Last Ronin", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 6}}},
		{"Garruk, Curse Breaker", []cardset.Entry{
			{Name: "Garruk, Curse Breaker", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 14}}},
		{"Jace, Reality Sculptor", []cardset.Entry{
			{Name: "Jace, Reality Sculptor", Count: 6}, {Name: "Wind Drake", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Lightning Bolt", Count: 2}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 4}}},
	} {
		deck := c.deck
		if !lockstepDiverges(t, "atkemblem", [2][]cardset.Entry{deck, foe}, 64) &&
			!lockstepDiverges(t, "atkemblem", [2][]cardset.Entry{foe, deck}, 64) {
			t.Errorf("%s: ignoring the attack emblems left the lockstep clean", c.name)
		}
	}
}
