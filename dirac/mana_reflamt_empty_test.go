package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflAmtViewEmptyCandidates: a lowered reflected Amount$ source
// (Incubation Druid) is a planner unit whose view is a function of the board
// (prepReflAmt): with no land among its controller's permanents the reflection
// names no symbol and the source makes no mana (gorge's paymentPlanReflectedAlt
// has no alternative); once a Forest is there, even a tapped one (CR 106.7),
// the view is one mana of {G}.
func TestReflAmtViewEmptyCandidates(t *testing.T) {
	deck := []cardset.Entry{{Name: "Incubation Druid", Count: 1}, {Name: "Forest", Count: 19}}
	tbl, _, _, err := cardsettest.Set(t, "manariders").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeLowering: true})
	druid := sc.perm(id["Incubation Druid"], 0)
	s, k := sc.s, sc.k
	sl := sc.slot(druid)
	if !k.gen.has(id["Incubation Druid"], gfManaRefl) {
		t.Fatal("precondition: Incubation Druid is not a reflected planner unit")
	}
	if pc := s.manaPCGen(k, sl); pc.manaAb >= 0 {
		t.Errorf("a Druid with no land has view %+v, want none (no mana)", pc)
	}
	sc.perm(id["Forest"], 0)
	pc := s.manaPCGen(k, sl)
	if pc.manaAb < 0 || pc.manaCol != kop.ColG || !pc.manaDyn || pc.manaN != 1 {
		t.Errorf("a Druid beside a Forest has view %+v, want one {G} from an evaluated-amount unit", pc)
	}
}
