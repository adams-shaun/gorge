package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeTwoPilesBFFixTime is gorge cr-twopiles-battlefield-validcards
// (01fc61aa7): before it the battlefield TwoPiles shape moved nothing.
const gorgeTwoPilesBFFixTime = "20261026114050"

// The Liliana lockstep: Liliana of the Veil's -6 (Ext
// TwoPilesSac: the controller splits the target player's permanents, that
// player picks the pile they sacrifice), beside her +1 and -2, both
// players running her so the target is either seat. The expected answer at
// every decision is gorge's.
var klilianaDecks = [2][]cardset.Entry{
	{
		{Name: "Liliana of the Veil", Count: 12}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Liliana of the Veil", Count: 8}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestLilianaLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeTwoPilesBFFixTime) {
		t.Skip("gorge before cr-twopiles-battlefield-validcards (01fc61aa7)")
	}
	lockstepSet(t, "liliana", klilianaDecks, 600, "KLILIANA_SEEDS")
}

// TestLilianaLockstepMutation: with the piles dropped, some seed diverges.
func TestLilianaLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeTwoPilesBFFixTime) {
		t.Skip("gorge before cr-twopiles-battlefield-validcards (01fc61aa7)")
	}
	n := lockstepRun(t, "liliana", klilianaDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtTwoPilesSac {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the piles dropped")
	}
}
