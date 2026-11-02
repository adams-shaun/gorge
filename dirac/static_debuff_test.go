package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDebuffOverride: an effect's override removing flying (gorge
// effDebuff, OpOverride of a RemoveKw override) puts the table on the
// layer-6 walk; the flyer loses flying, and a newer until-end-of-turn
// grant of flying gives it back (timestamp order).
func TestDebuffOverride(t *testing.T) {
	deck := []cardset.Entry{{Name: "Healer's Hawk", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	hc := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Healer's Hawk" {
			hc = CardID(i)
		}
	}
	tbl.Overrides = append(tbl.Overrides, kop.Override{RemoveKw: kop.KwFlying})
	ov := int16(len(tbl.Overrides) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen.fxNeed&fxNeedL6 == 0 {
		t.Fatal("an override's RemoveKw did not put the table on the layer-6 walk")
	}
	s := sc.s
	hawk := sc.perm(hc, 0)
	sl := sc.slot(hawk)
	if s.kw(sc.k, sl)&kop.KwFlying == 0 {
		t.Fatal("the hawk does not fly")
	}
	s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, hawk, hc
	s.res.e.srcVer = s.objs[hawk].ver
	s.exec(sc.k, &kop.Instr{Op: kop.OpOverride, Obj: kop.RefSelf, C: ov, A: kop.Const(kop.DurEOT)})
	if s.kw(sc.k, sl)&kop.KwFlying != 0 {
		t.Fatal("debuffed: the hawk still flies")
	}
	s.grantEOT(sl, kop.KwFlying)
	s.promoKw(sc.k, sl, kop.KwFlying)
	if s.kw(sc.k, sl)&kop.KwFlying == 0 {
		t.Fatal("a newer grant of flying: the hawk does not fly")
	}
}
