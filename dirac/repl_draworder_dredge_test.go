package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 draw-order ask (dirac/repl_draworder.go) beside the dredge ask
// (dirac/kw_dredge.go): a draw that decides the dredge ask first then meets a
// competition of draw-replacement programs, at the turn draw and on an OpDraw
// card, must keep the order ask pending across the dredge answer's tail. The
// shapes below are the ones the dredge path used to drop.

// drawOrderDredgeScen is vanilla's table plus two any-player ReplDrawProgram
// replacements (seat 0 gains its controller 3 life, seat 1 makes the drawer
// lose 1), a Dredge 4 card and a {0} "draw two, gain 100" spell; both
// replacements are on the battlefield, seat 0 is active.
func drawOrderDredgeScen(t *testing.T) (sc *scen, gainObj, loseObj Obj, thug, spell CardID) {
	t.Helper()
	tb := newSPTable()
	repl := func(p ...kop.Instr) CardID {
		tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Replacement", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer}, Prog: p}}})
		return CardID(len(tb.tbl.Cards) - 1)
	}
	gain := repl(kop.Instr{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)})
	lose := repl(kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)})
	thug = CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Dredge", Types: kop.TCreature, Dredge: 4})
	spell = CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Two", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)},
			{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(100)},
		}}}})
	sc = tb.scen(t)
	gainObj = sc.perm(gain, 0)
	loseObj = sc.perm(lose, 1)
	if !sc.onBF(gainObj) || !sc.onBF(loseObj) {
		t.Fatal("precondition: both draw replacements must be on the battlefield")
	}
	if !drawOrderOn(sc.k) || !dredgeOn(sc.k) {
		t.Fatal("precondition: the table lacks a draw program or dredge use")
	}
	return sc, gainObj, loseObj, thug, spell
}

// drawOrderResSpell runs the spell program through the resolver the way a
// resolving spell does (drawOrderScen's state).
func drawOrderResSpell(sc *scen, spell CardID, obj Obj) {
	s := sc.s
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: obj,
		srcVer: s.objs[obj].ver, card: spell, ab: 0, ctrl: 0},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.runProg(sc.k)
}

// TestReplDrawOrderDredgeTurnDraw: declining the turn draw's dredge ask poses
// the order ask; the answer applies the picked program and finishes the draw
// step (not the priority decision over a pending ask).
func TestReplDrawOrderDredgeTurnDraw(t *testing.T) {
	sc, _, _, thug, _ := drawOrderDredgeScen(t)
	s := sc.s
	sc.grave(thug, 0, 1)
	if s.dredgeCands(sc.k, 0, nil) != 1 {
		t.Fatal("precondition: seat 0 has no dredge candidate")
	}
	toDrawStep(sc)
	dredgeAsk(t, sc, 2)                    // the turn draw's dredge ask
	s.Apply(MakeAction(AkChoose, 0, 0, 1)) // decline: the ordinary draw meets the competition
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d, want the order ask", s.g.stalled)
	}
	if s.res.ask != raGenDrawOrder {
		t.Fatalf("res.ask = %d, want raGenDrawOrder %d", s.res.ask, raGenDrawOrder)
	}
	if s.ctl != ctlGenAsk || !s.g.drawFirst {
		t.Fatalf("ctl %d drawFirst %v: the draw step was finished over the order ask", s.ctl, s.g.drawFirst)
	}
	life, hand := int(s.pl[0].life), s.pl[0].handN
	d := sc.expect(t, DkChoose, ChGenReplOrder)
	if d.Player != 0 || d.N != 2 {
		t.Fatalf("player %d options %d, want seat 0's two-way order ask", d.Player, d.N)
	}
	s.Apply(d.Options[0]) // the gain program
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d after the order answer", s.g.stalled)
	}
	if got := int(s.pl[0].life); got != life+3 {
		t.Fatalf("life = %d, want %d (the picked replacement applied)", got, life+3)
	}
	if s.pl[0].handN != hand {
		t.Fatalf("hand = %d, want %d (the draw was replaced)", s.pl[0].handN, hand)
	}
	if s.ctl != ctlPriority || s.g.drawFirst {
		t.Fatalf("ctl %d drawFirst %v, want the draw step's priority after the answer", s.ctl, s.g.drawFirst)
	}
}

// TestReplDrawOrderDredgeOpDraw: declining card one's dredge ask in a Draw-2
// resolution poses the order ask, and the loop waits for it instead of asking
// card two's dredge question over it; both cards are replaced by their picks.
func TestReplDrawOrderDredgeOpDraw(t *testing.T) {
	sc, gainObj, _, thug, spell := drawOrderDredgeScen(t)
	s := sc.s
	sc.grave(thug, 0, 1)
	if s.dredgeCands(sc.k, 0, nil) != 1 {
		t.Fatal("precondition: seat 0 has no dredge candidate")
	}
	drawOrderResSpell(sc, spell, s.perms[sc.slot(gainObj)].obj)
	if s.prog(sc.k)[s.res.pc].Op != kop.OpDraw {
		t.Fatal("precondition: the resolution is not paused on the OpDraw")
	}
	life, hand := int(s.pl[0].life), s.pl[0].handN
	dredgeAsk(t, sc, 2)                    // card one's dredge ask
	s.Apply(MakeAction(AkChoose, 0, 0, 1)) // decline
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d, want the order ask", s.g.stalled)
	}
	d := drawOrderAsk(t, sc) // not card two's dredge ask
	if d.Player != 0 || d.N != 2 {
		t.Fatalf("player %d options %d, want seat 0's two-way order ask", d.Player, d.N)
	}
	s.applyResolveGen(sc.k, d.Options[0]) // the gain program, card one
	if got := int(s.pl[0].life); got != life+3 {
		t.Fatalf("life = %d, want %d (card one's picked replacement applied)", got, life+3)
	}
	if s.res.ask != raDredge {
		t.Fatalf("res.ask = %d, want card two's dredge ask %d", s.res.ask, raDredge)
	}
	dredgeAsk(t, sc, 2)                    // card two's dredge ask
	s.Apply(MakeAction(AkChoose, 0, 0, 1)) // decline
	d = drawOrderAsk(t, sc)
	s.applyResolveGen(sc.k, d.Options[1]) // the lose-life program, card two
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d after both cards were answered", s.g.stalled)
	}
	if got, want := int(s.pl[0].life), life+3-1+100; got != want {
		t.Fatalf("life = %d, want %d (both cards' picks then the spell's next instruction)", got, want)
	}
	if s.pl[0].handN != hand {
		t.Fatalf("hand = %d, want %d (both draws were replaced)", s.pl[0].handN, hand)
	}
}

// TestReplDrawOrderDredgeBodyStall: picking the plain ReplDrawMore of a mixed
// competition runs its body batch in the draw's place (drawMore), and that
// batch's first dredge ask (raDredgeRes, rfBodyTurn) is an ask the order
// answer cannot wait behind; the declared StallDrawRepl is latched rather
// than finishing the draw step over a pending ask.
func TestReplDrawOrderDredgeBodyStall(t *testing.T) {
	tb := newSPTable()
	more := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw More", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
	gain := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Gain Body", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)}}}}})
	thug := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Dredge", Types: kop.TCreature, Dredge: 4})
	sc := tb.scen(t)
	plainObj := sc.perm(more, 0)
	if !sc.onBF(plainObj) || !sc.onBF(sc.perm(gain, 1)) {
		t.Fatal("precondition: both draw replacements must be on the battlefield")
	}
	s := sc.s
	sc.grave(thug, 0, 1)
	if s.dredgeCands(sc.k, 0, nil) != 1 {
		t.Fatal("precondition: seat 0 has no dredge candidate")
	}
	if c := s.drawScan(sc.k, 0, drawChain{}, false); c.more != 1 || !c.prog || c.all != 2 {
		t.Fatalf("candidates more %d prog %v all %d, want one plain ReplDrawMore beside the program", c.more, c.prog, c.all)
	}
	toDrawStep(sc)
	dredgeAsk(t, sc, 2)                    // the turn draw's dredge ask
	s.Apply(MakeAction(AkChoose, 0, 0, 1)) // decline
	d := sc.expect(t, DkChoose, ChGenReplOrder)
	picked := -1
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(plainObj) {
			picked = i
		}
	}
	if picked < 0 {
		t.Fatalf("the plain ReplDrawMore is not among the %d order options", d.N)
	}
	s.Apply(d.Options[picked])
	if s.res.ask != raDredgeRes || s.res.flags&rfBodyTurn == 0 {
		t.Fatalf("res.ask %d flags %x, want the picked body's dredge ask raDredgeRes with rfBodyTurn", s.res.ask, s.res.flags)
	}
	if s.g.stalled != StallDrawRepl {
		t.Fatalf("stalled = %d, want StallDrawRepl %d: the body's ask was not waited behind", s.g.stalled, StallDrawRepl)
	}
}

// TestReplDrawOrderMixedOneMore: one plain ReplDrawMore beside a program is a
// real competition (the order ask offers both); the program's body draw then
// meets the plain replacement through the draw's lineage.
func TestReplDrawOrderMixedOneMore(t *testing.T) {
	tb := newSPTable()
	more := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw More", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
	prog := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Program", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer},
			Prog: []kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)},
			}}}})
	sc := tb.scen(t)
	plainObj := sc.perm(more, 0)
	progObj := sc.perm(prog, 0)
	if !sc.onBF(plainObj) || !sc.onBF(progObj) {
		t.Fatal("precondition: both replacements must be on the battlefield")
	}
	if c := sc.s.drawScan(sc.k, 0, drawChain{}, false); c.more != 1 || !c.prog || c.all != 2 {
		t.Fatalf("candidates more %d prog %v all %d, want one plain ReplDrawMore beside the program", c.more, c.prog, c.all)
	}
	s := sc.s
	life, hand := int(s.pl[0].life), s.pl[0].handN
	toDrawStep(sc)
	d := sc.expect(t, DkChoose, ChGenReplOrder)
	picked := -1
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(progObj) {
			picked = i
		}
	}
	if picked < 0 {
		t.Fatalf("the program is not among the %d order options", d.N)
	}
	s.Apply(d.Options[picked])
	if s.g.stalled != 0 {
		t.Fatalf("stalled = %d, want the program's body draw met the plain replacement", s.g.stalled)
	}
	if got, want := int(s.pl[0].life), life+3; got != want {
		t.Fatalf("life = %d, want %d (the program's body applied)", got, want)
	}
	if got, want := int(s.pl[0].handN), int(hand)+2; got != want {
		t.Fatalf("hand = %d, want %d (the body's draw was replaced by two)", got, want)
	}
}

// TestReplDrawOrderMixedMoreStall: two plain ReplDrawMore beside a program is
// not the commutative pair drawMore settles, so it keeps StallDrawRepl.
func TestReplDrawOrderMixedMoreStall(t *testing.T) {
	tb := newSPTable()
	more := func() CardID {
		tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw More", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
		return CardID(len(tb.tbl.Cards) - 1)
	}
	a, b := more(), more()
	prog := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Program", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)}}}}})
	sc := tb.scen(t)
	sc.perm(a, 0)
	sc.perm(b, 0)
	sc.perm(prog, 1)
	if c := sc.s.drawScan(sc.k, 0, drawChain{}, false); c.more != 2 || !c.prog || c.all != 3 {
		t.Fatalf("candidates more %d prog %v all %d, want two plain ReplDrawMore beside the program", c.more, c.prog, c.all)
	}
	toDrawStep(sc)
	sc.s.Step()
	if sc.s.g.stalled != StallDrawRepl {
		t.Fatalf("stalled = %d, want StallDrawRepl %d", sc.s.g.stalled, StallDrawRepl)
	}
}

// TestReplDrawOrderOpDrawLongCountStalls: the resumable loop's state holds the
// count in one byte. A program table's OpDraw longer than the loop can resume
// is a declared stall (before the order ask it took the plain path and drew
// the whole count).
func TestReplDrawOrderOpDrawLongCountStalls(t *testing.T) {
	tb := newSPTable()
	prog := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Program", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)}}}}})
	spell := CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Many", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(256)}}}}})
	sc := tb.scen(t)
	if !drawOrderOn(sc.k) || dredgeOn(sc.k) {
		t.Fatal("precondition: the table must be a draw-program table without dredge")
	}
	drawOrderResSpell(sc, spell, sc.obj(prog, 1))
	if sc.s.g.stalled != StallDrawRepl {
		t.Fatalf("stalled = %d, want StallDrawRepl %d (the loop cannot hold a 256-card draw)", sc.s.g.stalled, StallDrawRepl)
	}
}
