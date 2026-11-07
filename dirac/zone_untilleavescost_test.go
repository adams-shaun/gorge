package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCostReturnAsEntersAskIsIdempotent: the as-enters ask a cost's returned
// card poses (preReturnCost, zone_untilleavescost.go) is rebuilt identically
// when a state stepped at it is stepped again (search/instkeys.go's
// CopyFrom + StepInto), and its answer enters the card under its owner with
// the chosen number (CR 614.12, 610.3).
func TestCostReturnAsEntersAskIsIdempotent(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	card := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Number Enters", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram},
			Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtChooseNumber)), B: kop.Const(1), C: 20}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.has(card, gfEnterRepl) {
		t.Fatal("the fixture table has no as-enters number program")
	}
	o := sc.obj(card, 1)
	sc.s.place(o, kop.ZoneExile, false)
	if sc.s.objs[o].zone != kop.ZoneExile {
		t.Fatalf("the returning card is in zone %v, want exile", sc.s.objs[o].zone)
	}
	// the parked ask of an activation's sacrificed exiler, outside a resolution
	sc.s.g.pre = preAsk{obj: o, kind: akNumber, state: preReturnCost}
	sc.s.ctl = ctlPriority
	first := sc.s.Step()
	if first.Kind != DkChoose || first.Sub != ChGenNumber || first.Player != 1 || first.N != 20 {
		t.Fatalf("first ask kind=%d sub=%d pl=%d n=%d, want DkChoose/ChGenNumber pl=1 n=20", first.Kind, first.Sub, first.Player, first.N)
	}

	// a copy of the stepped state poses the same ask, not a priority decision
	var cp State
	cp.CopyFrom(sc.s)
	again := cp.Step()
	if again.Kind != first.Kind || again.Sub != first.Sub || again.Player != first.Player || again.N != first.N {
		t.Fatalf("re-stepped ask kind=%d sub=%d pl=%d n=%d, want kind=%d sub=%d pl=%d n=%d", again.Kind, again.Sub, again.Player, again.N, first.Kind, first.Sub, first.Player, first.N)
	}
	if cp.g.pre.state != preReturnCost || cp.ctl != ctlGenAsk {
		t.Fatalf("re-stepped park state=%d ctl=%d, want state=%d ctl=%d", cp.g.pre.state, cp.ctl, preReturnCost, ctlGenAsk)
	}

	// the answer enters the card under its owner with the chosen number
	answer := MakeAction(AkChoose, 0, 0, 7)
	sc.s.Apply(answer)
	if sc.s.objs[o].zone != kop.ZoneBattlefield || sc.s.objs[o].owner() != 1 {
		t.Fatalf("answered card zone=%v owner=%d, want battlefield under 1", sc.s.objs[o].zone, sc.s.objs[o].owner())
	}
	if sc.s.chosenNumberOf(sc.s.objs[o].slot) != 7 {
		t.Fatalf("chosen number %d, want 7", sc.s.chosenNumberOf(sc.s.objs[o].slot))
	}
	if sc.s.g.pre.state != 0 || sc.s.ctl != ctlPriority {
		t.Fatalf("after the answer park state=%d ctl=%d, want 0/%d", sc.s.g.pre.state, sc.s.ctl, ctlPriority)
	}
}
