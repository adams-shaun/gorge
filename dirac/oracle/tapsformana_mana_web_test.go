package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mana Web's TapsForMana trigger is the pool's controller-scoped watcher
// (ValidCard$ Land.OppCtrl) whose body taps the trigger land's controller's
// lands that can produce a type of mana the triggering land could
// (dirac/kgen lower_tapsformana.go, dirac/filter_samemanatype.go). The decks
// give the watcher's side Islands and Mountains and the tapper's side
// Mountains (a shared type, so lands tap), Forests (no shared type) and
// creatures to keep the game moving.
func manaWebDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Mana Web", Count: 4},
			{Name: "Island", Count: 10},
			{Name: "Mountain", Count: 6},
			{Name: "Sol Ring", Count: 8},
			{Name: "Grizzly Bears", Count: 8},
			{Name: "Shock", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 12},
			{Name: "Llanowar Elves", Count: 8},
			{Name: "Sol Ring", Count: 4},
			{Name: "Mountain", Count: 8},
			{Name: "Forest", Count: 8},
		},
	}
}

// TestTapsForManaManaWebLockstep replays the pool's games with Mana Web's
// trigger and production filter in the kernel's table.
func TestTapsForManaManaWebLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsformana", manaWebDecks(), 150, "TAPSMANA_MANAWEB_SEEDS")
}

// dropManaWebTapsLine drops Mana Web's TapsForMana trigger from the kernel's
// table (the mutation test's mutator).
func dropManaWebTapsLine(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		if cards[i].Name != "Mana Web" {
			continue
		}
		keep := cards[i].Abilities[:0:0]
		for _, a := range cards[i].Abilities {
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps &&
				a.Trig.Flags5&kop.Tf5TapsForMana != 0 {
				continue
			}
			keep = append(keep, a)
		}
		cards[i].Abilities = keep
	}
	tbl.Cards = cards
}

// TestTapsForManaManaWebMutation proves the games exercise Mana Web's
// trigger: with its line dropped, some seed diverges (a green lockstep alone
// would pass a pool where the line never fires).
func TestTapsForManaManaWebMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "tapsformana", manaWebDecks(), 150, false, dropManaWebTapsLine)
	if n == 0 {
		t.Fatal("no seed diverged with Mana Web's TapsForMana line dropped")
	}
}
