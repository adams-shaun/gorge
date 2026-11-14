package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC protection-1 lockstep: Protection Gains$ Choice, Choices$ AnyColor
// (Mother of Runes, Gods Willing, Shelter, Benevolent Bodyguard, Emerge
// Unscathed, Alseid of Life's Bounty, Aven Liberator) lowered onto
// ChooseColorRes + ColorGrant: the target creature or enchantment gains
// protection from the colour its controller picks until end of turn, beside
// the opposing seat's red and black creatures, burn and removal that the
// protection blocks, prevents or targets around. The expected answer at
// every decision is gorge's.
var kmbcprotection1Decks = [2][]cardset.Entry{
	{
		{Name: "Mother of Runes", Count: 4}, {Name: "Gods Willing", Count: 3},
		{Name: "Shelter", Count: 3}, {Name: "Benevolent Bodyguard", Count: 3},
		{Name: "Emerge Unscathed", Count: 3}, {Name: "Alseid of Life's Bounty", Count: 3},
		{Name: "Aven Liberator", Count: 2}, {Name: "Savannah Lions", Count: 4},
		{Name: "Serra Angel", Count: 2}, {Name: "Plains", Count: 13},
	},
	{
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Terror", Count: 3},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 10},
	},
}

func TestMBCProtection1Lowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcprotection1", kmbcprotection1Decks, 400, "KMBCPROTECTION1_SEEDS")
}

// TestMBCProtection1LowersMutation: with the chosen-colour grant turned into
// a no-op, some seed diverges, so the pool resolves the protection and the
// kernel prices it.
func TestMBCProtection1LowersMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "mbcprotection1", kmbcprotection1Decks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		hit := 0
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for p := range prog {
					if prog[p].Ext() == kop.ExtColorGrant && prog[p].C&4 != 0 {
						prog[p].C = 0
						hit++
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
		if hit == 0 {
			t.Errorf("no chosen-colour protection ExtColorGrant in the table")
		}
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never resolves the chosen protection")
	}
}
