package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ChangeZone Defined$ ValidGraveyard <filter> out of the graveyard (kgen
// lower_zone.go definedValidGraveyard: the ChangeZoneAll sweep of <filter>):
// Faith's Reward and Second Sunrise return what died this turn, No Rest for
// the Wicked returns it to hand, Pyrrhic Revival every creature card with a
// -1/-1 counter. The Bears trade in combat, so graveyards fill.
var definedValidGraveDecks = [2][]cardset.Entry{
	{
		{Name: "Faith's Reward", Count: 6}, {Name: "Second Sunrise", Count: 4}, {Name: "Pyrrhic Revival", Count: 2}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "No Rest for the Wicked", Count: 4}, {Name: "Second Sunrise", Count: 4},
		{Name: "Swamp", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
	},
}

func TestDefinedValidZoneLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "definedvalidgrave", definedValidGraveDecks, 80, true, nil)
}

// TestDefinedValidZoneLockstepMutation: with every graveyard sweep dropped
// (the deck's only ChangeZoneAll carriers are these four), some seed diverges.
func TestDefinedValidZoneLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "definedvalidgrave", definedValidGraveDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					if prog[k].Op == kop.OpChangeZoneAll {
						prog[k].Op = kop.OpNop
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the ValidGraveyard sweeps dropped")
	}
}
