package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferChosenGapAsksPrintedColoursFirst: a Taps-deferred
// chosen-colour land (the Core rewritten to Thriving Grove's "Combo G
// Chosen") is asked in gorge's order: the printed colour, then the chosen
// one (pay.SubstituteChosenProduced rewrites the last token). With white
// chosen the ask is G, W while the engine's order is W, G, so answer 0
// floats {G} and answer 1 floats {W}. The kernel once read the mask in WUBRG
// order and floated the chosen colour first.
func TestTapsDeferChosenGapAsksPrintedColoursFirst(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) {
			a.ManaColors = kop.ColG
			a.AFlags |= kop.AfManaChosen
		})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	core := sc.perm(id["Firdoch Core"], 0)
	sc.perm(id["Judge of Currents"], 0)
	sl := sc.slot(core)
	sc.s.perms[sl].chosen = kop.W
	if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaChosen || pc.manaCol != kop.ColG {
		t.Fatalf("precondition: the Core's mana facts are chosen %v col %#x, want a chosen-colour source printing G", pc.manaChosen, pc.manaCol)
	}
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the chosen-colour Core is no script gap")
	}
	if got := sc.s.deferGapAnswers(sc.k, sl); got != 2 {
		t.Fatalf("%d colour answers, want 2 (G and the chosen W)", got)
	}
	var a Action // no announced colours: the ask keeps gorge's token order
	gap := slotMask{}.with(sl)
	for c, want := range []uint8{kop.G, kop.W} {
		s := sc.s.fork()
		before := s.pl[0].pool
		var spill [kop.NumManaKinds]uint8
		s.scriptGapTap(sc.k, 0, a, sl, slotMask{}, slotMask{}, slotMask{}, gap, slotMask{}, c, &spill)
		exp := before
		exp[want]++
		if s.pl[0].pool != exp {
			t.Errorf("answer %d floated pool %v, want one {%d} added to %v", c, s.pl[0].pool, want, before)
		}
		sc.s.unfork()
	}
}
