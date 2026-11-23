package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestImprintedWordTakesItsSource: the !IsImprinted read must get the filter's
// source object like the positive word does. A card matcher (wordCard2) only
// called cardFilterSrc for W7Imprinted, so the negation always read the
// resolution's set: a trigger or static whose source is not the resolving
// entry saw the wrong list. Here the exiled card is imprinted by A, so A's
// !IsImprinted is false and another source B's is true.
func TestImprintedWordTakesItsSource(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (srcMemScen's trick)
	host := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "T Imprint Host", Types: kop.TCreature, Power: 2, Toughness: 2, ImpPersist: true})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "T Exiled Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredWord7, PredN: kop.W7Imprinted})
	imp := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredWord7, PredN: kop.W7NotImprinted})
	not := kop.FilterID(len(tbl.Filters) - 1)

	sc := newScenTable(t, &tbl, Config{NoElide: true})
	a := sc.perm(host, 0)
	b := sc.perm(host, 1)
	o := sc.obj(bear, 0)
	sc.s.place(o, kop.ZoneExile, false)
	if !sc.s.impSource(sc.k, a) {
		t.Fatal("the table did not keep an Imprinted list: the precondition is missing")
	}
	sc.s.osAdd(osImp, a, o)
	ctxA := &evalCtx{src: a, srcSl: sc.slot(a), ctrl: 0}
	ctxB := &evalCtx{src: b, srcSl: sc.slot(b), ctrl: 1}
	match := func(f kop.FilterID, cx *evalCtx) bool {
		return sc.s.matchCardGenAt(sc.k, f, bear, 0, 0, o, cx)
	}
	if !match(imp, ctxA) {
		t.Fatal("precondition: the exiled card is A's imprint")
	}
	if match(imp, ctxB) {
		t.Fatal("precondition: the exiled card is not B's imprint")
	}
	if match(not, ctxA) {
		t.Error("!IsImprinted read by the imprinting source must be false")
	}
	if !match(not, ctxB) {
		t.Error("!IsImprinted read by another source must be true")
	}
}
