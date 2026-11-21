package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Activator$ You / Opponent shapes behind the stack TapsForMana triggers
// (dirac/kgen lower_tapsformana.go), played against gorge over the tapsformana
// fixture pool: Sanctimony gains a life when an opponent taps a Mountain,
// Savage Firecat loses a +1/+1 counter when its controller taps a land and
// Treasure Nabber takes control of an artifact an opponent tapped for mana.
// Vorinclex, Voice of Hunger does both: its controller's land taps add a
// mana of the type produced and an opponent's land taps freeze the land.
// Each is a deferred payment tier on the taps the Activator$ gate reaches
// (the kernel's tapsWatchClass), so the casts both seats make exercise the
// gate on both engines.
//
// TestTapsForManaActivatorLockstep replays the pool's games per card; the
// mutation test drops the card's TapsForMana line from the kernel's table and
// expects divergence, so the seeds do exercise it.
func tapsForManaActivatorDecks() map[string][2][]cardset.Entry {
	return map[string][2][]cardset.Entry{
		// Sanctimony is {1W}: its seat taps only Plains and Wastes-free white
		// mana, and the opponent's Mountains are the ones the trigger watches
		// (Activator$ Opponent, ValidCard$ Mountain); the opponent's shocks
		// and its own fires keep both seats casting once the trigger defers
		// the watched Mountains.
		"sanctimony": {
			{
				{Name: "Sanctimony", Count: 4}, {Name: "Samite Healer", Count: 4},
				{Name: "Plains", Count: 16}, {Name: "Shock", Count: 8}, {Name: "Sol Ring", Count: 8},
			},
			{
				{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 16},
				{Name: "Shock", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Sol Ring", Count: 4},
			},
		},
		// Savage Firecat is {3RR}: its controller's own land taps remove its
		// +1/+1 counters (Activator$ You, ValidCard$ Land), so its lands all
		// defer in its census while it is in play, and the opponent plays a
		// plain green deck.
		"firecat": {
			{
				{Name: "Savage Firecat", Count: 4}, {Name: "Mountain", Count: 16},
				{Name: "Shock", Count: 12}, {Name: "Sol Ring", Count: 8},
			},
			{
				{Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 16},
				{Name: "Giant Growth", Count: 8}, {Name: "Llanowar Elves", Count: 6},
			},
		},
		// Treasure Nabber is {2R}: the artifacts its opponent taps for mana
		// (rocks of every production shape) are taken until the end of the
		// Nabber controller's next turn, so the stolen rock's census moves
		// with the control records.
		"nabber": {
			{
				{Name: "Treasure Nabber", Count: 4}, {Name: "Mountain", Count: 20},
				{Name: "Shock", Count: 8}, {Name: "Sol Ring", Count: 4}, {Name: "Doom Blade", Count: 4},
			},
			{
				{Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Arcane Signet", Count: 4},
				{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
				{Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 6},
			},
		},
		// Vorinclex is {6GG}: rocks and elves ramp it out, then its controller's
		// Forests double (the Static$ True You line) and the opponent's
		// Mountains it taps stay tapped (the Opponent line, ValidCard$ Land).
		"vorinclex": {
			{
				{Name: "Vorinclex, Voice of Hunger", Count: 6}, {Name: "Forest", Count: 16},
				{Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Arcane Signet", Count: 4},
				{Name: "Llanowar Elves", Count: 6},
			},
			{
				{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 16},
				{Name: "Shock", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Sol Ring", Count: 4},
			},
		},
	}
}

// dropTapsForManaLine drops the named card's TapsForMana trigger line from the
// kernel's table (the mutation test's mutator).
func dropTapsForManaLine(tbl *kop.Table, name string) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		if cards[i].Name != name {
			continue
		}
		var keep []kop.Ability
		for _, a := range cards[i].Abilities {
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags5&kop.Tf5TapsForMana != 0 {
				continue
			}
			keep = append(keep, a)
		}
		cards[i].Abilities = keep
	}
	tbl.Cards = cards
}

// TestTapsForManaActivatorLockstep replays the pool's games per card; the
// mutation test drops the card's TapsForMana line from the kernel's table and
// expects divergence, so the seeds do exercise it.
func TestTapsForManaActivatorLockstep(t *testing.T) {
	t.Parallel()
	decks := tapsForManaActivatorDecks()
	t.Run("sanctimony", func(t *testing.T) {
		t.Parallel()
		lockstepSet(t, "tapsformana", decks["sanctimony"], 150, "TAPSMANA_ACT_SEEDS")
	})
	t.Run("firecat", func(t *testing.T) {
		t.Parallel()
		lockstepSet(t, "tapsformana", decks["firecat"], 150, "TAPSMANA_ACT_SEEDS")
	})
	t.Run("nabber", func(t *testing.T) {
		t.Parallel()
		lockstepSet(t, "tapsformana", decks["nabber"], 150, "TAPSMANA_ACT_SEEDS")
	})
	t.Run("vorinclex", func(t *testing.T) {
		t.Parallel()
		lockstepSet(t, "tapsformana", decks["vorinclex"], 150, "TAPSMANA_ACT_SEEDS")
	})
}

// TestTapsForManaActivatorMutation: with the card's own TapsForMana line
// dropped from the kernel's table, some seed diverges, so each deck pair does
// exercise its card's trigger (a green lockstep alone would pass a pool where
// the line never fires).
func TestTapsForManaActivatorMutation(t *testing.T) {
	t.Parallel()
	for name, decks := range tapsForManaActivatorDecks() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := lockstepRun(t, "tapsformana", decks, 200, false, func(tbl *kop.Table) {
				dropTapsForManaLine(tbl, map[string]string{
					"sanctimony": "Sanctimony",
					"firecat":    "Savage Firecat",
					"nabber":     "Treasure Nabber",
					"vorinclex":  "Vorinclex, Voice of Hunger",
				}[name])
			})
			if n == 0 {
				t.Fatal("no seed diverged with the card's TapsForMana line dropped")
			}
		})
	}
}
