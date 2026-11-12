package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Monstrous Hound lockstep: the CantAttack line's "can't attack unless
// defending player controls fewer lands than you" (an UnlessDefender$
// hasFewerLandsInPlayThanYou lowered to the defender's count at least the
// controller's) and the CantBlock line's "can't block unless the attacking
// player controls fewer lands than you and it's not your turn" (NotPlayerTurn
// + CheckSVar$ Y SVarCompare$ GEX), with Mogg Toady and Goblin Goon the
// creature-count twins. The expected answer at every decision is gorge's.
var khoundDecks = [2][]cardset.Entry{
	{
		{Name: "Monstrous Hound", Count: 4}, {Name: "Mogg Toady", Count: 4}, {Name: "Goblin Goon", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	},
}

func TestHoundLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "hound", khoundDecks, 240, "KHOUND_SEEDS")
}

// TestHoundLockstepMutation: with the attack or the block gate's condition
// dropped, some seed diverges, so the pool attacks and blocks with the
// restricted creatures and the kernel prices their counts.
func TestHoundLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		kind kop.StaticKind
	}{
		{"no attack gate", kop.StatCantAttack},
		{"no block gate", kop.StatCantBlock},
	} {
		n := lockstepRun(t, "hound", khoundDecks, 120, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == m.kind {
						abs[j].Static.Cond2 = 0
					}
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never exercises the gate", m.name)
		}
	}
}
