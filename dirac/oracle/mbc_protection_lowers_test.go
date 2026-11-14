package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC protection-2 lockstep: an Aura's granted protection from
// creatures (Spirit Mantle, Unquestioned Authority: AddKeyword$
// Protection:Creature on the enchanted creature, a layer-6 grant of the
// kernel's Static.ProtFrom) and Akroma's Will's PumpAll "protection from each
// color" until end of turn (ExtColorGrant with every colour's mark), beside
// the opposing seat's creatures, burn and removal that the protection
// blocks, prevents or targets around. The expected answer at every decision
// is gorge's.
var kmbcprotection2Decks = [2][]cardset.Entry{
	{
		{Name: "Spirit Mantle", Count: 4}, {Name: "Unquestioned Authority", Count: 4},
		{Name: "Akroma's Will", Count: 3}, {Name: "Savannah Lions", Count: 5},
		{Name: "Serra Angel", Count: 3}, {Name: "Silver Knight", Count: 1},
		{Name: "Plains", Count: 20},
	},
	{
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Terror", Count: 3},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 10},
	},
}

func TestMBCProtection2Lowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcprotection2", kmbcprotection2Decks, 400, "KMBCPROTECTION2_SEEDS")
}

// TestMBCProtection2LowersMutation: with the granted-protection quality and
// the each-color marks dropped, some seed diverges, so the pool plays the
// protection and the kernel prices it.
func TestMBCProtection2LowersMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "mbcprotection2", kmbcprotection2Decks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.ProtFrom != 0 {
					abs[j].Static.ProtFrom = 0
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never plays a granted protection")
	}
}

// TestMBCProtection2LowersEachColorMutation: with Akroma's Will's every-colour
// marks (ExtColorGrant cgProtAll) turned into a no-op, some seed diverges.
func TestMBCProtection2LowersEachColorMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "mbcprotection2", kmbcprotection2Decks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		hit := 0
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for p := range prog {
					if prog[p].Ext() == kop.ExtColorGrant && prog[p].C&8 != 0 {
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
			t.Errorf("no each-color ExtColorGrant in the table")
		}
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never resolves Akroma's Will's protection")
	}
}
