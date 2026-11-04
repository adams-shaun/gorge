package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// trekvalue: Captain James T. Kirk's enters-or-attacks charm chooses one
// mode, or one or more with no cards in hand (CharmNum$ Count$Hellbent.3.1
// on a triggered charm, its count read as the trigger is put on the stack,
// CR 603.3c, 700.2); the cheap creatures and Divination empty both hands.
// Encounter the Unknown deals 2 to each creature and each player; Fevered
// Visions reads the active player's hand on an opponent's turn; Gul Dukat,
// Colonial Prefect repeats over the opponent's life lost this turn; Jean-Luc
// Picard, Luminary draws for another historic permanent of mana value 3 or
// more (Kirk, O'Brien, Gul Dukat); Chief Miles O'Brien doubles counters on
// the second draw of a turn.
var trekvalueDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Captain James T. Kirk", Count: 4}, {Name: "Jean-Luc Picard, Luminary", Count: 2}, {Name: "Chief Miles O'Brien", Count: 3},
			{Name: "Encounter the Unknown", Count: 3}, {Name: "Fevered Visions", Count: 2}, {Name: "Divination", Count: 3},
			{Name: "Savannah Lions", Count: 4}, {Name: "Raging Goblin", Count: 4},
			{Name: "Mountain", Count: 6}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 4},
		},
		{
			{Name: "Gul Dukat, Colonial Prefect", Count: 2}, {Name: "Captain James T. Kirk", Count: 4}, {Name: "Encounter the Unknown", Count: 3},
			{Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Memnite", Count: 4},
			{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
		},
	},
}

func TestTrekValueLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	_, games, _ := trekRun(t, "trekvalue", trekvalueDecks, 100, false)
	t.Logf("%d games", games)
}

// TestTrekValueMutation: with every dynamic charm count read as one, Kirk's
// trigger with an empty hand offers a single mode where gorge offers up to
// three.
func TestTrekValueMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCharmCmpCap = true
	defer func() { dirac.DebugCharmCmpCap = false }()
	if _, _, diverged := trekRun(t, "trekvalue", trekvalueDecks, 100, true); !diverged {
		t.Fatal("no divergence in 100 seeds with Kirk's charm count read as one")
	}
}
