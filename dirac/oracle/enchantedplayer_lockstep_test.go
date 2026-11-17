package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Player.EnchantedBy, the player the source Curse enchants (kop.WhoseEnchanted,
// kernel defEnchanted): Cruel Reality's and Curse of Obsession's Phase lines
// fire on that player's turn only, Curse of Stalked Prey's DamageDone line on
// combat damage to that player only. Both decks attack with Bears, and Curse
// of Obsession may enchant either player.
var enchantedPlayerDecks = [2][]cardset.Entry{
	{
		{Name: "Curse of Stalked Prey", Count: 6}, {Name: "Cruel Reality", Count: 4}, {Name: "Curse of Obsession", Count: 4}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 14}, {Name: "Curse of Obsession", Count: 4}, {Name: "Curse of Stalked Prey", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8},
	},
}

func TestFilterHeadPlayerLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "enchantedplayer", enchantedPlayerDecks, 80, true, nil)
}

// TestFilterHeadPlayerLockstepMutation: with the enchanted-player gate read as
// any player, some seed diverges.
func TestFilterHeadPlayerLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "enchantedplayer", enchantedPlayerDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Trig.Whose == kop.WhoseEnchanted {
					abs[j].Trig.Whose = kop.WhoseAny
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with Player.EnchantedBy read as any player")
	}
}
