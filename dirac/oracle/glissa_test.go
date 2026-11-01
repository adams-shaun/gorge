package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeRemoveAnyFixTime is gorge cr-removecounter-any-upto (461d8b818):
// before it CounterType$ Any removed nothing.
const gorgeRemoveAnyFixTime = "20261026095710"

// The Glissa lockstep: Glissa Sunslayer's third mode, "remove up
// to three counters from target permanent" (Ext RemoveCounterAny: a kind
// pick per counter, Done at every step), against +1/+1 counters (Luminarch
// Aspirant, Travel Preparations) and -1/-1
// counters (Contagion Clasp). The expected answer at every decision is
// gorge's.
var kglissaDecks = [2][]cardset.Entry{
	{
		{Name: "Glissa Sunslayer", Count: 10}, {Name: "Travel Preparations", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Luminarch Aspirant", Count: 6}, {Name: "Travel Preparations", Count: 4}, {Name: "Contagion Clasp", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 6},
	},
}

func TestGlissaLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeRemoveAnyFixTime) {
		t.Skip("gorge before cr-removecounter-any-upto (461d8b818)")
	}
	lockstepSet(t, "glissa", kglissaDecks, 600, "KGLISSA_SEEDS")
}

// TestGlissaLockstepMutation: with the removal dropped, some seed diverges.
func TestGlissaLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeRemoveAnyFixTime) {
		t.Skip("gorge before cr-removecounter-any-upto (461d8b818)")
	}
	n := lockstepRun(t, "glissa", kglissaDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtRemoveCounterAny {
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
		t.Fatal("no seed diverged with the counter removal dropped")
	}
}
