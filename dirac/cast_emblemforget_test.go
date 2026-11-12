package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEmbFlashForgetReadsTheEmblemsPlayerAsYou: a Quicken-shaped emblem
// (Filter2 "Sorcery.YouCtrl") ends at its own player's sorcery cast only.
// gorge effectCastSweep matches the cast against the effect's controller, so an
// opponent's sorcery leaves the grant standing.
func TestEmbFlashForgetReadsTheEmblemsPlayerAsYou(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TSorcery, Ctrl: kop.CtrlYou, Zone: kop.ZoneStack})
	f2 := kop.FilterID(len(tbl.Filters) - 1)
	sorc := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "sorcery", Types: kop.TSorcery})
	tbl.Emblems = append([]kop.Emblem{}, v.Table.Emblems...)
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{{
		Kind:   kop.AbStatic,
		Static: kop.Static{Kind: kop.StatCastWithFlash, Filter2: f2},
	}}})
	idx := uint8(len(tbl.Emblems))
	sc := newScenTable(t, &tbl, Config{NoElide: true})

	for _, c := range []struct {
		name string
		ctrl uint8
		want uint8
	}{{"opponent's sorcery", 1, 1}, {"own sorcery", 0, 0}} {
		embDurPut(sc, 0, NoObj, idx)
		if sc.s.g.nemb[0] != 1 || sc.k.t.Emblems[idx-1].Abilities[0].Static.Filter2 != f2 {
			t.Fatal("precondition: the Quicken-shaped emblem is not in seat 0's slot")
		}
		sc.s.embFlashForget(sc.k, &stackEntry{card: sorc, ctrl: c.ctrl, obj: NoObj})
		if got := sc.s.g.nemb[0]; got != c.want {
			t.Errorf("%s: %d emblems left, want %d", c.name, got, c.want)
		}
	}
}
