package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The chooser counter-cost lockstep: gorge's AddCounter<N/M1M1/
// Creature.YouCtrl> payment, lowered to a Blight cost (dirac/kgen/lower.go).
// Seat 0 carries the 4 cards — the spells (Lethal Sting, Scarscale Ritual)
// and the activated abilities (Hatchet Bully, Wandering Mage) — over
// creatures the picks pay with; seat 1's removal keeps the board moving.
// The expected answer at every decision is gorge's.
var kcountercostDecks = [2][]cardset.Entry{
	{
		{Name: "Hatchet Bully", Count: 4}, {Name: "Lethal Sting", Count: 4},
		{Name: "Scarscale Ritual", Count: 4}, {Name: "Wandering Mage", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Raging Goblin", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
		{Name: "Plains", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Raging Goblin", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 6},
	},
}

func TestCounterCostsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.CostBlightStats
	dirac.DebugCostBlightStats = &st
	defer func() { dirac.DebugCostBlightStats = nil }()
	lockstepSet(t, "countercosts", kcountercostDecks, 1000, "KCOUNTERCOST_SEEDS")
	t.Logf("countercosts: %d spell-paid, %d ability-paid blight costs", st.Spells.Load(), st.Abilities.Load())
	if testing.Short() {
		return
	}
	if s, a := st.Spells.Load(), st.Abilities.Load(); s == 0 || a == 0 {
		t.Fatalf("the pool barely pays the cost: %d spell-paid, %d ability-paid (both want > 0)", s, a)
	}
}
