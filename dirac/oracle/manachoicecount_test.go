package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A colour choice whose amount is counted when the effect resolves: Muerra
// (a Combo, one pick per Raccoon), Grand Warlord Radha (a Combo of the
// attackers, kept through the turn), the Shrines (one colour for the whole
// count) and Culling Ritual (a Combo of the permanents it destroyed).
var kmanachoiceDecks = [2][]cardset.Entry{{
	{Name: "Muerra, Trash Tactician", Count: 4}, {Name: "Grand Warlord Radha", Count: 4}, {Name: "Culling Ritual", Count: 3},
	{Name: "Llanowar Elves", Count: 3}, {Name: "Walking Ballista", Count: 2}, {Name: "Hill Giant", Count: 2},
	{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4},
}, {
	{Name: "Sanctum of Fruitful Harvest", Count: 4}, {Name: "Sanctum of Stone Fangs", Count: 4}, {Name: "Muerra, Trash Tactician", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Walking Ballista", Count: 2}, {Name: "Culling Ritual", Count: 2},
	{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
}}

func TestManaChoiceCountLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "manachoicecount", kmanachoiceDecks, 300, "KMANACHOICECOUNT_SEEDS")
}

// TestManaChoiceCountMutation: with the colour choices dropped, some seed
// diverges from gorge.
func TestManaChoiceCountMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "manachoicecount", kmanachoiceDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpExt && p[x].A == kop.Const(int(kop.ExtManaChoice)) {
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
		t.Fatal("no seed diverged with the mana choices dropped")
	}
}
