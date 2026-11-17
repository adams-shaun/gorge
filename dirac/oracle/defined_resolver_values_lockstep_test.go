package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Defined$ `A & B` over two players (dirac/kgen lower_defineduniononplayers.go):
// Karazikar, the Eye Tyrant's second trigger draws and drains "the attacking
// player and you". The kernel is two-seat, where an opponent cannot attack
// "another one of your opponents", so that trigger never fires here; the
// lockstep proves the card, with its lowered union, plays decision for
// decision beside gorge through its first trigger (tap and goad the player's
// creature on every attack) and combat.
func definedResolverValuesDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Karazikar, the Eye Tyrant", Count: 6},
		{Name: "Accursed Marauder", Count: 14},
		{Name: "Swamp", Count: 10},
		{Name: "Mountain", Count: 10},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestDefinedResolverValuesLockstep(t *testing.T) {
	t.Parallel()
	lockstepRunPolicy(t, "mbc", definedResolverValuesDecks(), 60, true, nil, nil, func(seed uint64) Policy {
		return RandomPolicy(seed)
	})
}
