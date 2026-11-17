package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The DamageDone self prevention gated on a +1/+1 counter (kgen selfDamageRepl,
// kernel selfDamage): Undergrowth Champion and Oathsworn Knight prevent damage
// to themselves while they have a +1/+1 counter and remove one, Ugin's
// Conjurant removes as many as the damage; with no counter left the damage is
// dealt. Shock and the Bears deal the damage.
var prevRemoveCtrDecks = [2][]cardset.Entry{
	{
		{Name: "Undergrowth Champion", Count: 5}, {Name: "Oathsworn Knight", Count: 5}, {Name: "Ugin's Conjurant", Count: 5},
		{Name: "Forest", Count: 13}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestDamageDoneReplShapeLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "prevremovectr", prevRemoveCtrDecks, 80, true, nil)
}

// TestDamageDoneReplShapeLockstepMutation: with the +1/+1 counter gate dropped
// (the damage always prevented), some seed diverges.
func TestDamageDoneReplShapeLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "prevremovectr", prevRemoveCtrDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbReplacement && abs[j].Repl.Kind == kop.ReplPreventDamageCounters {
					abs[j].Repl.C &^= kop.ReplPrevSelfIfCounter
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the +1/+1 counter gate dropped")
	}
}
