package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSupportBlinkedSourceIsOffered: support's "other" excludes the
// incarnation whose ability is resolving. A source that was blinked in
// response is a new object (CR 400.7), so the creature on the battlefield
// under its id is another creature and is offered (gorge's +Other does).
func TestSupportBlinkedSourceIsOffered(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear := CardID(len(tbl.Cards))
	sup := []kop.Instr{{Op: kop.OpSupport, A: kop.Const(2), C: int16(kop.CtrP1P1) | 0x100}}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	src := sc.perm(bear, 0)
	s.res.e.obj, s.res.e.ctrl, s.res.e.srcVer = src, 0, s.objs[src].ver
	offered := func() bool {
		var d Decision
		s.supportOptions(k, &sup[0], &d)
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkChoose && Obj(a.Source()) == src {
				return true
			}
		}
		return false
	}
	if offered() {
		t.Fatal("precondition: the resolving incarnation must not be offered")
	}
	s.res.e.srcVer-- // the trigger was put on the stack by the previous incarnation
	if !offered() {
		t.Fatal("a re-entered source is another creature and must be offered")
	}
}
