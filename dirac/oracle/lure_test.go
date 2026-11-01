package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeLureFixTime is gorge cr-lure-all-must-block (617daacfb, 2026-10-26
// UTC): before it the lure sentence was unread.
const gorgeLureFixTime = "20261026102225"

// The lure lockstep: Prized Unicorn's "All creatures able to
// block CARDNAME do so." (CR 509.1c) against a defender with ground
// creatures, Giant Growth to make blocks matter. The expected answer at
// every decision is gorge's.
var klureDecks = [2][]cardset.Entry{
	{
		{Name: "Prized Unicorn", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 6},
		{Name: "Forest", Count: 18},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6},
		{Name: "Prized Unicorn", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 10},
	},
}

func TestLureLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLureFixTime) {
		t.Skip("gorge before cr-lure-all-must-block (617daacfb)")
	}
	lockstepSet(t, "lure", klureDecks, 800, "KLURE_SEEDS")
}

// TestLureLockstepMutation: with the lure dropped, some seed diverges.
func TestLureLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLureFixTime) {
		t.Skip("gorge before cr-lure-all-must-block (617daacfb)")
	}
	n := lockstepRun(t, "lure", klureDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			cards[i].Lure = false
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the lure dropped")
	}
}
