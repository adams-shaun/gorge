package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 order ask of competing Draw replacement programs
// (dirac/repl_draworder.go): two ReplDrawProgram bodies matching one draw do
// not commute, so the drawing player picks which applies, as gorge's
// poseReplacementChoice does. The ask is posed at the sites a draw can be
// suspended on (an OpDraw card, the turn draw); any other draw keeps the
// StallDrawRepl stall.

// drawOrderScen is a table with two any-player draw replacements: host 0
// gains its controller 3 life, host 1 makes the drawer lose 1, each in the
// draw's place. Both are on the battlefield (seats 0 and 1).
func drawOrderScen(t *testing.T, prog []kop.Instr) (*scen, []kop.Instr) {
	t.Helper()
	tb := newSPTable()
	repl := func(p ...kop.Instr) CardID {
		tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Replacement", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer}, Prog: p}}})
		return CardID(len(tb.tbl.Cards) - 1)
	}
	gain := repl(kop.Instr{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)})
	lose := repl(kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)})
	spell := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Spell", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := tb.scen(t)
	a := sc.perm(gain, 0)
	b := sc.perm(lose, 1)
	if !sc.onBF(a) || !sc.onBF(b) {
		t.Fatal("precondition: both draw replacements must be on the battlefield")
	}
	s := sc.s
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: s.perms[sc.slot(a)].obj,
		srcVer: s.objs[s.perms[sc.slot(a)].obj].ver, card: spell, ab: 0, ctrl: 0},
		active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, prog
}

// drawOrderAsk is the posed order ask's decision.
func drawOrderAsk(t *testing.T, sc *scen) Decision {
	t.Helper()
	s := sc.s
	if s.res.ask != raGenDrawOrder {
		t.Fatalf("res.ask = %d (stalled %d), want the draw order ask %d: the competition must be asked, not capped (StallDrawRepl %d)",
			s.res.ask, s.g.stalled, raGenDrawOrder, StallDrawRepl)
	}
	var d Decision
	if !s.resolveOptionsGen(sc.k, &d) || d.Kind != DkChoose || d.Sub != ChGenReplOrder {
		t.Fatalf("decision kind %d sub %d, want DkChoose/ChGenReplOrder", d.Kind, d.Sub)
	}
	return d
}

func TestReplDrawOrderAskOpDraw(t *testing.T) {
	prog := []kop.Instr{
		{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)},
		{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(100)},
	}
	for _, first := range []int{0, 1} {
		sc, prog := drawOrderScen(t, prog)
		s := sc.s
		life, hand := s.pl[0].life, s.pl[0].handN
		s.runProg(sc.k)
		d := drawOrderAsk(t, sc)
		if d.Player != 0 || d.N != 2 {
			t.Fatalf("player %d options %d, want the drawer asked over both replacements", d.Player, d.N)
		}
		if s.pl[0].life != life || s.pl[0].handN != hand {
			t.Fatal("the draw was applied before the order was answered")
		}
		// the first draw: option `first` applies; the second draw asks again
		s.applyResolveGen(sc.k, d.Options[first])
		d = drawOrderAsk(t, sc)
		s.applyResolveGen(sc.k, d.Options[1-first])
		if s.res.ask != raNone || s.g.stalled != 0 {
			t.Fatalf("res.ask %d stalled %d after both draws were answered", s.res.ask, s.g.stalled)
		}
		// +3 for the gain replacement, -1 for the lose replacement, then the
		// spell's own GainLife: the resolution went on at the next instruction
		if got, want := int(s.pl[0].life), int(life)+3-1+100; got != want {
			t.Fatalf("first %d: life = %d, want %d", first, got, want)
		}
		if s.pl[0].handN != hand {
			t.Fatalf("first %d: hand = %d, want %d (both draws were replaced)", first, s.pl[0].handN, hand)
		}
		_ = prog
	}
}

func TestReplDrawOrderAskPicksTheOrder(t *testing.T) {
	// the pick decides which body replaces the draw: a lone draw
	for _, pick := range []int{0, 1} {
		sc, _ := drawOrderScen(t, []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}})
		s := sc.s
		life := int(s.pl[0].life)
		s.runProg(sc.k)
		d := drawOrderAsk(t, sc)
		s.applyResolveGen(sc.k, d.Options[pick])
		want := life + 3
		if pick == 1 {
			want = life - 1
		}
		if got := int(s.pl[0].life); got != want {
			t.Fatalf("pick %d: life = %d, want %d", pick, got, want)
		}
	}
}

func TestReplDrawOrderAskTurnDraw(t *testing.T) {
	sc, _ := drawOrderScen(t, nil)
	s := sc.s
	s.res = resCtx{} // the draw step has no resolution
	s.step, s.active, s.ctl = kop.StepUpkeep, 0, ctlStepBegin
	life := int(s.pl[0].life)
	s.step = kop.StepDraw
	s.beginStep(sc.k)
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d, want the turn draw's order asked (StallDrawRepl %d)", s.g.stalled, StallDrawRepl)
	}
	var d Decision
	if s.ctl != ctlGenAsk || !s.genAskOptions(sc.k, &d) || d.Sub != ChGenReplOrder || d.N != 2 || d.Player != 0 {
		t.Fatalf("ctl %d sub %d options %d player %d, want ctlGenAsk and the 2-way order ask of seat 0", s.ctl, d.Sub, d.N, d.Player)
	}
	s.applyGen(sc.k, d.Options[0])
	if s.ctl != ctlPriority || s.step != kop.StepDraw {
		t.Fatalf("ctl %d step %d, want the draw step's priority after the answer", s.ctl, s.step)
	}
	if got := int(s.pl[0].life); got != life+3 {
		t.Fatalf("life = %d, want %d (the picked replacement applied)", got, life+3)
	}
}
