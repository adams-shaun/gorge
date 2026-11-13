package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC effect-api-1 spelling family, in lockstep with gorge: the Effect
// cards whose ReplacementEffects$ kgen refused. One subtest per spelling; each
// plays a fixture pool whose table is refused (the pool does not build) while
// the spelling is unlowered. The expected answer at every decision is gorge's.

// Ethereal Haze (no ValidTarget$: every recipient), Surge of Salvation (a
// colour-filtered ValidSource$ onto creatures you control) and Loyal Unicorn
// (IsCombat$ True) against red burn and creatures.
var kmbcPreventDecks = [2][]cardset.Entry{
	{
		{Name: "Ethereal Haze", Count: 5}, {Name: "Surge of Salvation", Count: 5}, {Name: "Loyal Unicorn", Count: 3},
		{Name: "Savannah Lions", Count: 6}, {Name: "Serra Angel", Count: 4}, {Name: "Plains", Count: 17},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestMBCEffectApi1Lowers(t *testing.T) {
	t.Parallel()
	t.Run("prevention-scope", func(t *testing.T) {
		t.Parallel()
		lockstepSet(t, "mbceffectapi1", kmbcPreventDecks, 600, "KMBCEFFECTAPI1_SEEDS")
	})
}

// dropPrevScope empties the scope bits of every Ext PreventDamageScope the
// named card carries: the Effect still resolves but registers no prevention.
func dropPrevScope(name string) func(*kop.Table) {
	return func(tb *kop.Table) {
		for i := range tb.Cards {
			if tb.Cards[i].Name != name {
				continue
			}
			for j := range tb.Cards[i].Abilities {
				prog := tb.Cards[i].Abilities[j].Prog
				for k := range prog {
					if prog[k].Ext() == kop.ExtPreventDamageScope {
						prog[k].C = 0
					}
				}
			}
		}
	}
}

// TestMBCEffectApi1PreventionMutation proves each card's prevention is met:
// with that card's record dropped, some seed diverges from gorge.
func TestMBCEffectApi1PreventionMutation(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Ethereal Haze", "Surge of Salvation"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if lockstepRun(t, "mbceffectapi1", kmbcPreventDecks, 300, false, dropPrevScope(name)) == 0 {
				t.Fatalf("no seed diverged with %s's prevention dropped: the pool never has its damage prevented", name)
			}
		})
	}
}
