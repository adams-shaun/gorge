package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestIgnoreLegendRuleExempt: a printed S:Mode$ IgnoreLegendRule static (CR
// 704.5j, gorge legendGroups' legendRuleExempt skip) takes its matching
// permanents out of the legend scan: the blanket form (no ValidCard$)
// exempts a whole same-named pair, the creature-scoped form spares only the
// creature of a creature-and-noncreature pair (the remaining single
// candidate rules nothing), and without the static the pair is a legend
// group. The precondition each case pins is legendRule itself, the check the
// state-based pass poses the legend ask from.
func TestIgnoreLegendRuleExempt(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	creature := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	leg, sigil, gallery, box := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2), CardID(len(tbl.Cards)+3)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Legend", Types: kop.TCreature | kop.TLegendary, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Legend", Types: kop.TArtifact | kop.TLegendary},
		kop.CardDef{Name: "Test Gallery", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIgnoreLegendRule}}}},
		kop.CardDef{Name: "Test Box", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIgnoreLegendRule, Filter: creature}}}})

	for _, c := range []struct {
		name   string
		bearer CardID
		mixed  bool
		exempt bool
	}{
		{"blanket: the pair rules nothing", gallery, false, true},
		{"creature scope: only the creature is exempt", box, true, true},
		{"no static: the pair is a legend group", 0, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			sc.perm(leg, 0)
			second := leg // the sigil for the mixed pair, a second legend otherwise
			if c.mixed {
				second = sigil
			}
			sc.perm(second, 0)
			if c.bearer != 0 {
				sc.perm(c.bearer, 0)
			}
			if got := sc.s.legendRule(sc.k); got == c.exempt {
				t.Fatalf("legendRule=%v, want %v", got, !c.exempt)
			}
		})
	}
}
