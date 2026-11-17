package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNumTimesBoundPricesChosenColourExactly: a trigger's "pay {B} up to three
// times" announcement (ExtNumTimes) beside a Chosen-colour source whose colour
// is red. Gorge prices the count against the payment-aware potential
// (cumulative.go triggeredCostNumTimesAsk, potentialManaFor with the trigger
// cost as the payment), which makes exactly {R} from the source, so no {B} is
// payable and no count is offered. The plain over-bound priced the chosen
// production open in every colour and offered a count up to the announcement's maximum.
func TestNumTimesBoundPricesChosenColourExactly(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) {
			a.ManaColors = 0
			a.AFlags |= kop.AfManaChosen
		})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	core := sc.perm(id["Firdoch Core"], 0)
	sl := sc.slot(core)
	sc.s.perms[sl].chosen = objWord(kop.R)
	if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaChosen || pc.manaAb < 0 {
		t.Fatalf("precondition: the Core's mana facts are chosen %v ability %d, want a live chosen-colour source", pc.manaChosen, pc.manaAb)
	}
	// the plain over-bound must reach {B} here, or the test cannot tell the two bounds apart
	var plain avail
	sc.s.availPotential(sc.k, 0, &plain)
	if b := cost(0, kop.B); !plain.affords(&b, pipMaskOf(&b), 0) {
		t.Fatalf("precondition: the plain over-bound does not reach {B}")
	}
	var in kop.Instr
	in.C = 3<<4 | 0 // unit generic 0, AnnounceMax 3
	in.B.N = 1 << (3 * kop.B)
	sc.s.res.e.ctrl = 0
	if sc.s.numTimesStart(sc.k, &in) {
		if sc.s.g.pickLeft != 0 {
			t.Errorf("no announcement expected, pickLeft %d", sc.s.g.pickLeft)
		}
		return
	}
	t.Errorf("red chosen, {B} per time: the count is offered up to %d, gorge's payment-aware bound offers none", sc.s.g.pickLeft)
}
