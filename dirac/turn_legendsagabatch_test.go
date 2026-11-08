package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestLegendRuleAndSagaSacrificeAreOneBatch: a legend-rule ask and a Saga due
// its sacrifice are in the same state-based check (CR 704.3, gorge
// parkLegendChoice parks the pass's acts with the ask), so the Neva the legend
// rule removes still sees the Saga go (CR 603.10a): "whenever an enchantment
// you control is put into a graveyard from the battlefield" triggers for both
// Nevas, not only the one kept.
func TestLegendRuleAndSagaSacrificeAreOneBatch(t *testing.T) {
	set := cardsettest.Synth(t, "std-woe")
	names := []string{"Neva, Stalked by Nightmares", "The Princess Takes Flight"}
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-woe table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 2})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	keep := sc.perm(id[names[0]], 0)
	gone := sc.perm(id[names[0]], 0)
	saga := sc.perm(id[names[1]], 0)
	s.perms[sc.slot(saga)].setCtrN(s, int(kop.CtrLore), int(k.cards[id[names[1]]].sagaN))
	if !s.sagaDue(k, sc.slot(saga)) {
		t.Fatal("the Saga is not due its sacrifice: the test sees nothing")
	}
	if !s.sba(k) || s.ctl != ctlLegend {
		t.Fatalf("no legend ask posed: ctl %v", s.ctl)
	}
	s.npend = 0
	s.applyLegend(k, MakeAction(AkChoose, uint8(keep), 0, 0))
	if s.objs[saga].zone == kop.ZoneBattlefield || s.objs[gone].zone == kop.ZoneBattlefield {
		t.Fatalf("the legend and the Saga must both leave: zones %v %v", s.objs[gone].zone, s.objs[saga].zone)
	}
	var kept, ruled int
	for i := 0; i < int(s.npend); i++ {
		switch s.pend[i].obj {
		case keep:
			kept++
		case gone:
			ruled++
		}
	}
	if kept != 1 || ruled != 1 {
		t.Fatalf("pending Neva triggers: kept %d, ruled out %d; want one each", kept, ruled)
	}
}
