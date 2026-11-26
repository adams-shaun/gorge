package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Table.Overrides is the pool's list: a record no card of the table can apply
// (tableReach) must not turn the granted-type views on (prepLandGrant).
func TestLandGrantViewsIgnorePoolOnlyOverrides(t *testing.T) {
	forest := kop.SubMask{}.With(0)
	tbl := &kop.Table{
		Subtypes:  []string{"Forest"},
		Overrides: []kop.Override{{HasSub: true, AddSub: forest}},
		Cards:     make([]kop.CardDef, 1),
	}
	var g genPrep
	prepLandGrant(tbl, &prep{cards: make([]pcard, 1)}, &g)
	if g.blgOn {
		t.Fatal("blgOn set by an override no card of the table can apply")
	}
	tbl.Cards[0].Abilities = []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: 1}}}
	g = genPrep{}
	prepLandGrant(tbl, &prep{cards: make([]pcard, 1)}, &g)
	if !g.blgOn {
		t.Fatal("blgOn unset though a card of the table applies the granting override")
	}
}
