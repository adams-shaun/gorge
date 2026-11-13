package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// tymnaBuild is kwardpowBuild keeping the pool's expression table.
func tymnaBuild(t *testing.T, name, src string) (kop.CardDef, []kop.Expr, []refusal) {
	t.Helper()
	card, diags := cards.ParseBytes("tymna_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, pt.exprs, lw.cards[0].Refusals
}

// TestTymnaLifeCostLowersBesideTheDamageTally: Tymna the Weaver's postcombat
// trigger pays PayLife<X> and draws X, X the opponents dealt combat damage
// this turn: one IfPaid(PayLife) of the ExOppsLostLife N 2 expression, the
// Draw reading the same expression; the same cost on any other X body, on a
// Main1 trigger or beside a literal life part refuses by name.
func TestTymnaLifeCostLowersBesideTheDamageTally(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(tableCorpus(t), "cardsfolder", "t", "tymna_the_weaver.txt"))
	if err != nil {
		t.Skip("no gorge corpus: ", err)
	}
	src := string(b)
	const head = "PlayerCountRegisteredOpponents$HasPropertywasDealtCombatDamageThisTurn"
	if !strings.Contains(src, "SVar:X:"+head) || !strings.Contains(src, "Phase$ Main2") || !strings.Contains(src, "Cost$ PayLife<X>") {
		t.Fatalf("precondition: the corpus script no longer has the shape this test probes:\n%s", src)
	}
	if v, err := (&lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}).svarValue(head, 0); err != nil || v.Kind != vOppsLostLife || v.N != 2 {
		t.Fatalf("head value = %s, %v; want vOppsLostLife N 2", v, err)
	}
	d, exprs, rs := tymnaBuild(t, "Tymna the Weaver", src)
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	var pay, draw *kop.Instr
	for _, a := range d.Abilities {
		for i := range a.Prog {
			switch a.Prog[i].Op {
			case kop.OpIfPaid:
				pay = &a.Prog[i]
			case kop.OpDraw:
				draw = &a.Prog[i]
			}
		}
	}
	if pay == nil || draw == nil {
		t.Fatalf("no IfPaid/Draw in the trigger's program: %+v", d.Abilities)
	}
	if pay.C&0xf != kop.PayLife || pay.A.K != kop.VExpr {
		t.Fatalf("IfPaid kind %d A %+v, want PayLife of an expression", pay.C&0xf, pay.A)
	}
	if x := exprs[pay.A.N]; x.Kind != kop.ExOppsLostLife || x.N != 2 {
		t.Fatalf("life cost expr = %+v, want ExOppsLostLife N 2", x)
	}
	if draw.A != pay.A {
		t.Errorf("Draw amount %+v, want the cost's own expression %+v (not a constant)", draw.A, pay.A)
	}

	for _, p := range []struct{ name, src string }{
		{"other head", strings.Replace(src, "SVar:X:"+head, "SVar:X:PlayerCountRegisteredOpponents$HasPropertyLostLifeThisTurn", 1)},
		{"unlowered property", strings.Replace(src, "SVar:X:"+head, "SVar:X:PlayerCountRegisteredOpponents$HasPropertywasDealtDamageThisTurn", 1)},
		{"Main1 trigger", strings.Replace(src, "Phase$ Main2", "Phase$ Main1", 1)},
		{"beside a literal life part", strings.Replace(src, "Cost$ PayLife<X>", "Cost$ PayLife<2> PayLife<X>", 1)},
	} {
		if p.src == src {
			t.Fatalf("%s: the probe edit changed nothing", p.name)
		}
		_, _, rs := tymnaBuild(t, "Tymna the Weaver", p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, "life cost") && !strings.Contains(all, "PlayerCountRegisteredOpponents") {
			t.Errorf("%s: refusals %q, want a life-cost or head refusal", p.name, all)
		}
	}
}
