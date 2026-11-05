package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Dathon lockstep: Dathon, His Heart Open's "{2}: choose a counter on
// target enchantment you control, put another of that kind on it" (Ext
// PutCounterExisting) over Sagas (a lore counter), Support Mission (a quest
// counter on itself) and the +1/+1 counters it puts on creatures that enter,
// so an enchantment creature Saga holds two kinds. The expected answer at
// every decision is gorge's.
var kdathonDeck = []cardset.Entry{
	{Name: "Dathon, His Heart Open", Count: 6}, {Name: "Support Mission", Count: 4},
	{Name: "Summon: Fat Chocobo", Count: 4}, {Name: "Summon: G.F. Ifrit", Count: 4},
	{Name: "Amok Time", Count: 2}, {Name: "The Measure of a Man", Count: 2},
	{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
}

var kdathonDecks = [2][]cardset.Entry{kdathonDeck, kdathonDeck}

func TestDathonLockstep(t *testing.T) {
	t.Parallel()
	asked := dirac.PutExistingStats.Asked.Load()
	lockstepSet(t, "dathon", kdathonDecks, 600, "KDATHON_SEEDS")
	if dirac.PutExistingStats.Asked.Load() == asked {
		t.Fatal("no two-kind ask was posed: the pool does not exercise the pick")
	}
}

// TestDathonLockstepMutation: with the counter put dropped, some seed diverges.
func TestDathonLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "dathon", kdathonDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtPutCounterExisting {
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
		t.Fatal("no seed diverged with the counter put dropped")
	}
}

// The Dathon order lockstep: the recipient is an enchantment creature Saga
// under a counter doubler (Dr. Joseph M'Benga) and a plus-one (Brad Boimler,
// Eager Ensign), so Dathon's put is a CR 616.1 order competition: the kind
// pick is asked first, then the order.
var kdathonOrderDecks = [2][]cardset.Entry{
	{
		{Name: "Dathon, His Heart Open", Count: 5}, {Name: "Dr. Joseph M'Benga", Count: 4}, {Name: "Brad Boimler, Eager Ensign", Count: 4},
		{Name: "Summon: Fat Chocobo", Count: 4}, {Name: "Summon: G.F. Ifrit", Count: 4}, {Name: "Support Mission", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 3},
	},
	{{Name: "Plains", Count: 40}},
}

func TestDathonOrderLockstep(t *testing.T) {
	t.Parallel()
	asked, ordered := dirac.PutExistingStats.Asked.Load(), dirac.CtrOrderStats.Asked.Load()
	lockstepSet(t, "dathon", kdathonOrderDecks, 300, "KDATHON_SEEDS")
	if dirac.PutExistingStats.Asked.Load() == asked || dirac.CtrOrderStats.Asked.Load() == ordered {
		t.Fatal("the pool posed no kind ask or no order ask")
	}
}
