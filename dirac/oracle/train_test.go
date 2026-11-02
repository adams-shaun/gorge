package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Training lockstep: Hopeful Initiate's Training (CR
// 702.149a: whenever it attacks with another creature of greater power, a
// +1/+1 counter; gorge trigmatch AttacksMatches) and its "{2}{W}, remove two
// +1/+1 counters from among creatures you control: destroy target artifact
// or enchantment" (gorge RemoveAnyCounter, rules amongCounterAsk), beside
// Lions and Angels, against Ornithopters, Pacifism and burn. The expected
// answer at every decision is gorge's.
var ktrainDecks = [2][]cardset.Entry{
	{
		{Name: "Hopeful Initiate", Count: 12}, {Name: "Savannah Lions", Count: 6}, {Name: "Serra Angel", Count: 3},
		{Name: "Plains", Count: 19},
	},
	{
		{Name: "Ornithopter", Count: 6}, {Name: "Pacifism", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

func TestTrainLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "train", ktrainDecks, 1000, "KTRAIN_SEEDS")
}

// TestTrainLockstepMutation proves the pool trains and pays the counter
// cost: dropping the Training gate (every attack counts) or the cost's
// counter removal (a free {2}{W} destroy) each makes some seed diverge.
func TestTrainLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Ability)
	}{
		{"training", func(a *kop.Ability) {
			if a.Kind == kop.AbTriggered && a.Trig.N == kop.TrigNTraining {
				a.Trig.N = 0
			}
		}},
		{"counter cost", func(a *kop.Ability) {
			if a.Cost.Flags&kop.CostSubAmong != 0 {
				a.Cost.Flags &^= kop.CostSubAmong
				a.Cost.N = 0
			}
		}},
	} {
		n := lockstepRun(t, "train", ktrainDecks, 200, false, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					m.fn(&tbl.Cards[i].Abilities[j])
				}
			}
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged with it dropped: the pool never exercises it", m.name)
		}
	}
}
