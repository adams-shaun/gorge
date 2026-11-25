package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Forbidden Orchard's "Whenever you tap CARDNAME for mana, target opponent
// creates a 1/1 colorless Spirit creature token" is a self-line TapsForMana
// watcher. gorge's planner defers the source (a token body is no vetted payer
// damage), and the kernel mirrors it: the lowered self line classes its own
// tap (mana_tapsdefer.go tapsWatchClassForeign) and the deferred-source
// search answers the payer's own target ask as the queued trigger is placed
// (deferGapWalk's pass node -> resolvePrefixStack's build arm), then resolves
// the token before offering the play. The decks tap Orchards for mana and
// cast cheap spells so the tap and its token change decisions.
var orchardTapsDecks = [2][]cardset.Entry{
	{
		{Name: "Forbidden Orchard", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Sol Ring", Count: 4},
		{Name: "Opt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Forbidden Orchard", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Shock", Count: 8}, {Name: "Opt", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 10},
	},
}

// TestTapsForManaOrchardLockstep replays the pool's games: no divergence, the
// casts and activations both seats make.
func TestTapsForManaOrchardLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsformana", orchardTapsDecks, 120, "TAPSMANA_ORCHARD_SEEDS")
}

// TestTapsForManaOrchardMutation: with the Orchard's self TapsForMana line
// dropped from the kernel's table, some seed diverges, so the lockstep
// exercises the deferred-source tap and its token rather than passing on
// games that never tap the Orchard.
func TestTapsForManaOrchardMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "tapsformana", orchardTapsDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Name != "Forbidden Orchard" {
				continue
			}
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps &&
					a.Trig.Flags&kop.TfSelf != 0 && a.Trig.Flags5&kop.Tf5TapsForMana != 0 {
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with Forbidden Orchard's self TapsForMana line dropped")
	}
}
