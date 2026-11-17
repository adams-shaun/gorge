package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// kdiscardxScen: p0 holds Turbulent Dreams ({U}{U}, whose additional cost is
// the announced Discard<X/Card> beside no printed {X}) with two untapped
// Islands; p1 has a Mountain in play to keep the seats symmetrical.
func kdiscardxScen(t *testing.T, cfg Config) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Turbulent Dreams", Count: 1}, {Name: "Island", Count: 20},
		{Name: "Mountain", Count: 20}}
	set := cardsettest.Synth(t, "tail-26")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, cfg)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Mountain"], 1)
	return sc, sc.hand(id["Turbulent Dreams"], 0)
}

// TestDiscardXSpellUnplanned: gorge's payment planner declines a spell whose
// own Cost$ has an announced Discard<X/Spec> part beside no printed {X}
// (rules/pay classify.go PlanNonManaAdmissible: part.Announced declines
// "cost:discard", as it does an announced Sac<X>), so under the autopay
// surface Turbulent Dreams is offered only when the floating pool pays it --
// never as a planned play from untapped sources (featview.go
// planDeclinesAddCost). The kernel surface (no gorge config) offers it from
// the untapped Islands.
func TestDiscardXSpellUnplanned(t *testing.T) {
	sc, td := kdiscardxScen(t, Config{GorgeAutopaySurface: true, NoElide: true})
	if sc.s.FeatPlannedCast(0, td, 0) {
		t.Fatalf("FeatPlannedCast(Turbulent Dreams) = true; gorge's planner declines an announced Discard<X> cost")
	}
	if offersCast(sc.s, td) {
		t.Fatalf("autopay surface offers Turbulent Dreams with an empty pool")
	}
	sc, td = kdiscardxScen(t, Config{NoElide: true})
	if !offersCast(sc.s, td) {
		t.Fatalf("kernel surface does not offer Turbulent Dreams from untapped Islands")
	}
}
