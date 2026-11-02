package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Heartwood Crafter's {C} can't be spent to cast a spell from hand
// (RestrictValid$ CantCastSpellFromHand): it pays an activated ability, a
// spell cast from exile (Rimrock Knight after Stomp) and nothing cast from hand.
var kheartwoodDecks = [2][]cardset.Entry{{
	{Name: "Heartwood Crafter", Count: 8}, {Name: "Rimrock Knight", Count: 4}, {Name: "Walking Ballista", Count: 4},
	{Name: "Mind Stone", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Forest", Count: 9}, {Name: "Mountain", Count: 9},
}, {
	{Name: "Heartwood Crafter", Count: 6}, {Name: "Rimrock Knight", Count: 4}, {Name: "Walking Ballista", Count: 3},
	{Name: "Hill Giant", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
}}

func TestHeartwoodCrafterLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "heartwoodcrafter", kheartwoodDecks, 300, "KHEARTWOODCRAFTER_SEEDS")
}

// TestHeartwoodCrafterMutation: with the non-spell half of the restriction
// dropped, the Crafter's mana no longer pays an activated ability, which
// gorge's search finds.
func TestHeartwoodCrafterMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "heartwoodcrafter", kheartwoodDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbMana && abs[j].ManaRestrict != 0 {
					abs[j].AFlags &^= kop.AfManaNonSpell
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the non-spell half dropped")
	}
}
