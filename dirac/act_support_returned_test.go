package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSupportReturnedSourceIsOther: support's "other" (C&0x100) withholds the
// resolving source while it is the object the ability came from (its version
// is the entry's srcVer), and offers it once it left the battlefield and
// returned since: a new object (CR 400.7, gorge's SourceReturned).
func TestSupportReturnedSourceIsOther(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sup := kop.Instr{Op: kop.OpSupport, A: kop.Const(2), C: int16(kop.CtrP1P1) | 0x100}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	src, other := sc.perm(bear, 0), sc.perm(bear, 1)
	s.res.e.obj, s.res.e.ctrl, s.res.e.kind = src, 0, seTrigger
	offered := func() bool {
		var d Decision
		s.supportOptions(k, &sup, &d)
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkChoose && Obj(a.Source()) == src {
				return true
			}
		}
		return false
	}
	s.res.e.srcVer = s.objs[src].ver
	if !s.supportOK(k, &sup, s.objs[other].slot) {
		t.Fatal("precondition: the other creature must be eligible")
	}
	if offered() {
		t.Fatal("the source, unchanged since the ability went on the stack, is offered")
	}
	s.res.e.srcVer = s.objs[src].ver - 2 // it left the battlefield and came back
	if !offered() {
		t.Fatal("the source that left and returned is a new object and must be offered")
	}
}
