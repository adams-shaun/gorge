package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWasCastForetoldRead: PredWasCastSpell word 10 (Condition$ Foretold,
// gorge FlagForetold) holds only for a spell on the stack whose entry the
// foretell cast marked (trigPlForetold, cast.go): the same spell cast from
// exile by another permission, cast from hand, a copy of the foretold spell
// (copy_gen.go clears the mark) and an object that is no spell on the stack
// all read false; the negated word (10|16) is each one's complement.
func TestWasCastForetoldRead(t *testing.T) {
	var s State
	const o = Obj(3)
	pred := func(n int16) *kop.Filter { return &kop.Filter{Pred: kop.PredWasCastSpell, PredN: n} }
	push := func(prov uint8, pl uint8, of uint8) {
		s.sp = 1
		s.stack[0] = stackEntry{kind: seSpell, obj: o, ctrl: 0, prov: prov, trigPl: pl}
		s.objs[o].of = of
	}
	for _, c := range []struct {
		name string
		prov uint8
		pl   uint8
		of   uint8
		want bool
	}{
		{"foretold cast from exile", uint8(kop.ZoneExile), trigPlForetold, 0, true},
		{"cast from exile by another permission", uint8(kop.ZoneExile), 0, 0, false},
		{"cast from hand", uint8(kop.ZoneHand), 0, 0, false},
		{"a copy of the foretold spell (mark cleared)", uint8(kop.ZoneExile), 0, ofCopy, false},
		{"a copy carrying no cast origin", 0, trigPlForetold, ofCopy, false},
	} {
		push(c.prov, c.pl, c.of)
		if got := s.wasCastSpell(pred(10), o, 0); got != c.want {
			t.Errorf("%s: word 10 read %v, want %v", c.name, got, c.want)
		}
		if got := s.wasCastSpell(pred(10|16), o, 0); got == c.want {
			t.Errorf("%s: negated word 10 read %v, want the complement", c.name, got)
		}
	}
	s.sp = 0
	s.objs[o].of = 0
	if s.wasCastSpell(pred(10), o, 0) {
		t.Error("an object that is no spell on the stack read as foretold")
	}
}
