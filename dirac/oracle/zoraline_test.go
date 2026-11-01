package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Zoraline lockstep: Zoraline, Cosmos Caller's "whenever it
// enters or attacks, you may pay {W}{B} and 2 life; when you do, return
// target nonland permanent card with mana value 3 or less from your
// graveyard to the battlefield with a finality counter on it" (gorge's
// trigger-cost window over a mana-and-life cost, then the reflexive return;
// CR 122.1's finality counter exiles the returned creature instead of its
// dying), against removal that fills the graveyard. The expected answer at
// every decision is gorge's.
var kzoralineDecks = [2][]cardset.Entry{
	{
		{Name: "Zoraline, Cosmos Caller", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Walking Corpse", Count: 6},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 10},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 12},
	},
}

func TestZoralineLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "zoraline", kzoralineDecks, 1000, "KZORALINE_SEEDS")
}

// TestZoralineLockstepMutation proves the pool pays the life and meets the
// finality counter: dropping the cost's life, or the counter's exile, each
// makes some seed diverge.
func TestZoralineLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Instr)
	}{
		{"life", func(in *kop.Instr) {
			if in.Op == kop.OpIfPaid && in.C&0xF == kop.PayMana {
				in.F = 0
			}
		}},
		{"finality", func(in *kop.Instr) {
			if in.Op == kop.OpReturnEx && in.B.N == int16(kop.CtrFinality) {
				in.B.N = int16(kop.CtrP1P1)
			}
		}},
	} {
		n := lockstepRun(t, "zoraline", kzoralineDecks, 300, false, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					for k := range tbl.Cards[i].Abilities[j].Prog {
						m.fn(&tbl.Cards[i].Abilities[j].Prog[k])
					}
				}
			}
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged with it dropped: the pool never exercises it", m.name)
		}
	}
}
