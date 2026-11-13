package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestKickedMayPlayCastPaysTheChosenCount: a may-play cast's cost is built
// from the card's printed cost plus the multikicker payments the count ask
// settled (CR 702.43), not one payment. The lockstep class cannot reach a
// may-play Multikicker card today, so this is the one home of that path
// (legal_gen.go buildCostGenPre).
func TestKickedMayPlayCastPaysTheChosenCount(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append(append([]kop.CardDef(nil), v.Table.Cards...), kop.CardDef{
		Name:           "Test Multikicker",
		Types:          kop.TSorcery,
		Cost:           kop.ManaCost{Generic: 2},
		Multikicker:    kop.ManaCost{Generic: 1},
		HasMultikicker: true,
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	id := CardID(len(v.Table.Cards))
	o := sc.hand(id, 0)
	s := sc.s
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: id, ctrl: 0, flags: sfMayPlay | sfKicked}}

	// zero payments: exactly the printed cost (the old one-payment price).
	s.setMultikick(&s.bld().e, 0)
	if got, _, ok := s.buildCostGenPre(sc.k, false); !ok || got != (kop.ManaCost{Generic: 2}) {
		t.Fatalf("count 0: %+v ok=%v, want {2}", got, ok)
	}

	// three payments: {2} + 3 x {1}.
	s.setMultikick(&s.bld().e, 3)
	if got, _, ok := s.buildCostGenPre(sc.k, false); !ok || got != (kop.ManaCost{Generic: 5}) {
		t.Fatalf("count 3: %+v ok=%v, want {5}", got, ok)
	}
}
