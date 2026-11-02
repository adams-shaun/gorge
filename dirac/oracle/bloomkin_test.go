package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Flourishing Bloom-Kin lockstep: its turn-face-up trigger's
// chain, shared with Cultivate and Kodama's Reach (Fork in the Road puts
// the second land in the graveyard), and played through them, since a
// face-down cast and a turn-up are priced from floating mana only: it
// searches for up to two Forests, which go to the top of the library
// unshuffled and remembered (Shuffle$ False), then one of them onto the
// battlefield tapped and the other into the hand, then the shuffle. The
// expected answer at every decision is gorge's.
var kbloomkinDecks = [2][]cardset.Entry{
	{
		{Name: "Flourishing Bloom-Kin", Count: 4}, {Name: "Cultivate", Count: 6}, {Name: "Kodama's Reach", Count: 4},
		{Name: "Fork in the Road", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestBloomkinLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "bloomkin", kbloomkinDecks, 600, "KBLOOMKIN_SEEDS")
}

// TestBloomkinLockstepMutation: with the unshuffled first search dropped,
// some seed diverges.
func TestBloomkinLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "bloomkin", kbloomkinDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpSearchEx && p[x].B.N&kop.SrNoShuffle != 0 && kop.Zone(p[x].C&7) == kop.ZoneLibrary {
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
		t.Fatal("no seed diverged with the first search dropped")
	}
}
