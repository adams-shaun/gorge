package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// exilebfDecks: spells and an ability whose cost exiles a creature the payer
// controls (Exile<1/Creature>: Fear of Abduction's additional cost, Necrotic
// Fumes' spell cost, Altar of Bhaal's activated cost), beside vanilla creatures and basics.
var exilebfDecks = [2][]cardset.Entry{
	{
		{Name: "Fear of Abduction", Count: 4}, {Name: "Necrotic Fumes", Count: 3},
		{Name: "Altar of Bhaal", Count: 3}, {Name: "Walking Corpse", Count: 5}, {Name: "Savannah Lions", Count: 1}, {Name: "Child of Night", Count: 3},
		{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Fear of Abduction", Count: 2}, {Name: "Necrotic Fumes", Count: 2},
		{Name: "Altar of Bhaal", Count: 3}, {Name: "Walking Corpse", Count: 6}, {Name: "Savannah Lions", Count: 3}, {Name: "Child of Night", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

// A cost that exiles a permanent you control from the battlefield pays it with
// a pick of the matching permanents, exiling each as the cost is paid: nothing
// is sacrificed, so no sacrifice trigger fires and a replacement on dying does
// not apply (CR 118.3, 602.2b, 406.2). Both engines agree on the offers, the
// picks and the zones.
func TestExileBattlefieldCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "exilebf", exilebfDecks, 64, "KEXILEBF_SEEDS")
	lockstepSet(t, "exilebf", [2][]cardset.Entry{exilebfDecks[1], exilebfDecks[0]}, 64, "KEXILEBF_SEEDS")
}

// TestExileBattlefieldCostMutation: with the exile pick turned into a
// sacrifice, some seed diverges, so the pool reaches a paid exile cost.
func TestExileBattlefieldCostMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "exilebf", exilebfDecks, 64, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			cards[i].SpellCost.ExileBf = false
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				abs[j].Cost.ExileBf = false
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the exile pick turned into a sacrifice")
	}
}

// TestExileBattlefieldActivatedMutation: the same for an activated ability's
// cost alone (Altar of Bhaal), so the pool reaches that pay path too.
func TestExileBattlefieldActivatedMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "exilebf", exilebfDecks, 64, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbActivated {
					abs[j].Cost.ExileBf = false
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the activated ability's exile pick turned into a sacrifice")
	}
}
