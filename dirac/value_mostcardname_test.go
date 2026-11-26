package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMostCardName: ExMostCardName is the greatest number of battlefield
// objects matching the filter that share one face name (gorge count_ref.go
// countMostCardName, Forge's Count$MostCardName): a lone land reads 1, two
// same-named lands read 2, a third differently-named land does not move it,
// an opponent's same-named land counts (gorge scans every seat's battlefield,
// AliveFrom(0), with the spec's You read against the evaluating controller),
// and a creature outside the filter does not.
func TestMostCardName(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	tbl.Filters = append(append([]kop.Filter(nil), vanilla.Table.Filters...), kop.Filter{Types: kop.TLand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	loam, grove, peak, bears := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2), CardID(len(tbl.Cards)+3)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Loam", Types: kop.TLand},
		kop.CardDef{Name: "Test Grove", Types: kop.TLand},
		kop.CardDef{Name: "Test Peak", Types: kop.TLand},
		kop.CardDef{Name: "Test Bears", Types: kop.TCreature, Power: 1, Toughness: 1})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	count := func() int {
		x := kop.Expr{Kind: kop.ExMostCardName, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if got := count(); got != 0 {
		t.Fatalf("no match reads %d", got)
	}
	sc.perm(loam, 0)
	if got := count(); got != 1 {
		t.Fatalf("a lone land reads %d, want 1", got)
	}
	sc.perm(grove, 0)
	if got := count(); got != 1 {
		t.Fatalf("two different names read %d, want 1", got)
	}
	sc.perm(loam, 0)
	if got := count(); got != 2 {
		t.Fatalf("two Test Loams read %d, want 2", got)
	}
	sc.perm(peak, 0)
	if got := count(); got != 2 {
		t.Fatalf("a third name read %d, want 2 (the Loams)", got)
	}
	sc.perm(loam, 1) // the opponent's Test Loam counts too
	if got := count(); got != 3 {
		t.Fatalf("the opponent's land moved the count to %d, want 3", got)
	}
	sc.perm(bears, 0) // a creature outside the land filter does not
	if got := count(); got != 3 {
		t.Fatalf("a creature moved the count to %d, want 3", got)
	}

	// A third Test Loam on the evaluating seat puts that name 2 ahead of the
	// controller-relative read of the same game: the whole scan is one
	// battlefield population, gorge's AliveFrom(0) loop.
	sc.perm(loam, 0)
	if got := count(); got != 4 {
		t.Fatalf("the fourth Test Loam moved the count to %d, want 4", got)
	}

	// The tie: three Groves against four Loams still reads the Loams' 4.
	for i := 0; i < 3; i++ {
		if sc.perm(grove, uint8(i%2)) == NoObj {
			t.Fatal("precondition: the grove did not enter")
		}
	}
	if got := count(); got != 4 {
		t.Fatalf("three Groves beside four Loams read %d, want 4", got)
	}
}
