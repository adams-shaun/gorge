package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ClassBand$ static lockstep: Blacksmith's Talent's level 3
// "during your turn, equipped creatures you control have double strike and
// haste" and Caretaker's Talent's level 3 "creature tokens you control get
// +2/+2", the Class statics the ClassBand$ condition newly lowers, beside
// token makers, creatures and removal. The expected answer at every
// decision is gorge's.
var kclassbandDecks = [2][]cardset.Entry{
	{
		{Name: "Blacksmith's Talent", Count: 4}, {Name: "Mountain", Count: 17},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Goblin Instigator", Count: 3}, {Name: "Shock", Count: 4},
	},
	{
		{Name: "Caretaker's Talent", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 7},
		{Name: "Raise the Alarm", Count: 4}, {Name: "Goblin Instigator", Count: 3}, {Name: "Walking Corpse", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Doom Blade", Count: 4},
	},
}

func TestClassbandLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "classband", kclassbandDecks)
	p3Lockstep(t, "classband", reg, set, decks, tbl, 600)
}

// TestClassbandMutation proves the pool exercises the level gates: with
// every static's condition dropped from the table (the level-3 statics
// apply from level 1), some seed diverges from gorge.
func TestClassbandMutation(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "classband", kclassbandDecks)
	mut := *tbl
	mut.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	n := 0
	for i := range mut.Cards {
		abs := append([]kop.Ability(nil), mut.Cards[i].Abilities...)
		for j := range abs {
			if abs[j].Kind == kop.AbStatic && abs[j].Static.Cond2 != 0 {
				abs[j].Static.Cond2 = 0
				n++
			}
		}
		mut.Cards[i].Abilities = abs
	}
	if n == 0 {
		t.Fatal("no conditional static in the table")
	}
	for seed := uint64(1); seed <= 100; seed++ {
		m, err := NewMatchup(reg, [2]string{"kclassband-a", "kclassband-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, &mut, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges without the level gates (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with the Class level gates dropped: the pool does not exercise them")
}
