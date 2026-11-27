package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Optional$ True Draw replacement (repl_drawmay.go): the draw pipeline
// poses gorge's replChoiceDraw election (apply / decline, canon.go's
// KReplacement yes / no) where exactly ONE may replacement matches a draw,
// asked of the draw's player. An accept runs the replacement (the Obstinate
// Familiar skip, or the allowlist program); a decline draws the card normally
// with the declined replacement in the draw's lineage (CR 614.5: the same
// replacement must not catch its decline's draw). The turn draw asks through
// the ctlGenAsk route as dredge's does, an OpDraw card through the resolution
// ask; a draw at another site stalls (StallDrawRepl) rather than guess gorge's
// ask order.

func mayProg(c int16, p ...kop.Instr) kop.CardDef {
	return kop.CardDef{Name: "Test May Replacement", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: c}, Prog: p}}}
}

func addCard(tb *spTable, c kop.CardDef) CardID {
	tb.tbl.Cards = append(tb.tbl.Cards, c)
	return CardID(len(tb.tbl.Cards) - 1)
}

// drawMayAsk checks the pending decision is seat 0's draw-replacement
// election: ChGenDrawMay, yes (mode 1) then no (mode 0), both naming host.
func drawMayAsk(t *testing.T, sc *scen, host Obj) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenDrawMay || d.Player != 0 || int(d.N) != 2 {
		t.Fatalf("decision kind %d sub %d player %d with %d options, want seat 0's draw-replacement ask with 2",
			d.Kind, d.Sub, d.Player, d.N)
	}
	if d.Options[0].Mode() != 1 || d.Options[1].Mode() != 0 {
		t.Fatalf("options mode %d/%d, want yes (1) before no (0)", d.Options[0].Mode(), d.Options[1].Mode())
	}
	for i := 0; i < 2; i++ {
		if d.Options[i].Source() != TCode(host) {
			t.Fatalf("option %d source %d, want the replacement host %d", i, d.Options[i].Source(), host)
		}
	}
}

// replDrawMayScenarios runs as TestReplDrawProgramScenarios' "Optional$ True
// election" case.
func replDrawMayScenarios(t *testing.T) {
	t.Run("the turn draw's may skip asks; an accept skips and the step ends", func(t *testing.T) {
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		if !sc.onBF(h) {
			t.Fatal("precondition: the may replacement host is not on the battlefield")
		}
		if sc.k.gen == nil || !sc.k.gen.hasDrawProgram() {
			t.Fatal("precondition: the gen flag hasDrawProgram is not set")
		}
		hand, lib := sc.s.pl[0].handN, sc.s.pl[0].libLen()
		if lib == 0 {
			t.Fatal("precondition: empty library before the draw")
		}
		toDrawStep(sc)
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 1)) // yes: replace the draw
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the accepted skip drew nothing)", got, hand)
		}
		if got := sc.s.pl[0].libLen(); got != lib {
			t.Fatalf("library = %d, want %d (the accepted skip drew nothing from the library)", got, lib)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered skip stalled", sc.s.g.stalled)
		}
		if d := sc.s.Step(); d.Kind != DkPriority || sc.s.g.drawFirst {
			t.Fatalf("decision kind %d (drawFirst %v): want the draw step's priority after the skip",
				d.Kind, sc.s.g.drawFirst)
		}
	})
	t.Run("a decline draws the card normally, once", func(t *testing.T) {
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		if !sc.onBF(h) {
			t.Fatal("precondition: the may replacement host is not on the battlefield")
		}
		hand, lib := sc.s.pl[0].handN, sc.s.pl[0].libLen()
		if lib == 0 {
			t.Fatal("precondition: empty library before the draw")
		}
		toDrawStep(sc)
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 0)) // no: draw the card
		if got := sc.s.pl[0].handN; got != hand+1 {
			t.Fatalf("hand = %d, want %d (the declined draw was made)", got, hand+1)
		}
		if got := sc.s.pl[0].libLen(); got != lib-1 {
			t.Fatalf("library = %d, want %d (exactly one card left it)", got, lib-1)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the declined draw stalled", sc.s.g.stalled)
		}
		if d := sc.s.Step(); d.Kind != DkPriority || sc.s.g.drawFirst {
			t.Fatalf("decision kind %d (drawFirst %v): want the draw step's priority after the draw",
				d.Kind, sc.s.g.drawFirst)
		}
	})
	t.Run("a mandatory skip asks nothing", func(t *testing.T) {
		// The precondition the accept case depends on: the SAME line without
		// the may bit replaces the draw unasked (repl_drawprogram.go).
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip))
		sc := tb.scen(t)
		sc.perm(host, 0)
		hand, lib := sc.s.pl[0].handN, sc.s.pl[0].libLen()
		if lib == 0 {
			t.Fatal("precondition: empty library before the draw")
		}
		toDrawStep(sc)
		if d := sc.s.Step(); d.Kind != DkPriority || d.Kind == DkChoose {
			t.Fatalf("decision kind %d: want the step's priority, no ask", d.Kind)
		}
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the mandatory skip applied unasked)", got, hand)
		}
		if got := sc.s.pl[0].libLen(); got != lib {
			t.Fatalf("library = %d, want %d (the mandatory skip drew nothing from the library)", got, lib)
		}
	})
	t.Run("an OpDraw asks per card and the spell continues past the draw", func(t *testing.T) {
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		spell := addCard(tb, kop.CardDef{Name: "Test Draw Two Then Life", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)},
			}}}})
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		o := sc.hand(spell, 0)
		sc.castCard(t, o)
		sc.passBoth(t)                                  // the resolution runs to the first card's ask
		hand, life := sc.s.pl[0].handN, sc.s.pl[0].life // the spell is on the stack: its own card is out of hand
		// card 1: yes, the skip
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 1))
		// card 2: no, draw it
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 0))
		if got := sc.s.pl[0].handN; got != hand+1 {
			t.Fatalf("hand = %d, want %d (one card drawn, one skipped)", got, hand+1)
		}
		if got := sc.s.pl[0].life; got != life+5 {
			t.Fatalf("life = %d, want %d (the spell's GainLife ran after the draws)", got, life+5)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered batch stalled", sc.s.g.stalled)
		}
		if d := sc.s.Step(); d.Kind != DkPriority || sc.s.sp != 0 {
			t.Fatalf("decision kind %d, stack %d: want the spell resolved", d.Kind, sc.s.sp)
		}
	})
	t.Run("a draw at another site stalls rather than ask", func(t *testing.T) {
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		spell := addCard(tb, kop.CardDef{Name: "Test Life Spell", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)},
			}}}})
		sc := tb.scen(t)
		sc.perm(host, 0)
		// a resolution whose pc is NOT on an OpDraw asks nothing: the kernel
		// has no resumable continuation for this draw, so it stalls (the
		// KeptOuterPC cast harness's frame, pointed at the GainLife op)
		sc.s.res = resCtx{e: stackEntry{kind: seAbility, ctrl: 0}, active: true}
		sc.s.res.pc = 0 // the spell program's GainLife op is not an OpDraw
		_ = spell
		hand := sc.s.pl[0].handN
		sc.s.draw(0)
		if sc.s.g.stalled != StallDrawRepl {
			t.Fatalf("stalled = %d, want StallDrawRepl (%d)", sc.s.g.stalled, StallDrawRepl)
		}
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the stalled draw was not made)", got, hand)
		}
		sc.s.res = resCtx{}
	})
	t.Run("an accept runs the allowlist program body in the draw's place", func(t *testing.T) {
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawOptional,
			kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(2)}))
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		hand, lib, life := sc.s.pl[0].handN, sc.s.pl[0].libLen(), sc.s.pl[0].life
		if lib == 0 || life < 3 {
			t.Fatalf("precondition: library %d, life %d before the draw", lib, life)
		}
		toDrawStep(sc)
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 1))
		if got := sc.s.pl[0].life; got != life-2 {
			t.Fatalf("life = %d, want %d (the accepted body lost two)", got, life-2)
		}
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the body replaced the draw)", got, hand)
		}
		if ds := sc.s.drawState(); len(ds) == 0 || ds[0].active || ds[0].may {
			t.Fatalf("drawProgram record %+v: want cleared after the body finished", ds)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the accepted body stalled", sc.s.g.stalled)
		}
		if d := sc.s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d: want the draw step's priority after the body", d.Kind)
		}
	})
	t.Run("the OpDraw loop mark leaves the grave-drawn config bit alone", func(t *testing.T) {
		// regression: an OpDraw loop mark once shared cfgx bit 1<<1 with
		// cfgxGraveDrawn, so every OpDraw card cleared the table's Init-set
		// grave-drawn flag; the site is drawOrderSite's read now
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		spell := addCard(tb, kop.CardDef{Name: "Test Draw One", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
			}}}})
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		sc.s.cfgx |= cfgxGraveDrawn // as Init sets it on a table with a graveyard Drawn trigger
		o := sc.hand(spell, 0)
		sc.castCard(t, o)
		sc.passBoth(t)
		drawMayAsk(t, sc, sc.s.perms[sc.slot(h)].obj)
		sc.s.Apply(MakeAction(AkChoose, TCode(h), 0, 0))
		if sc.s.cfgx&cfgxGraveDrawn == 0 {
			t.Fatal("cfgxGraveDrawn was cleared by the OpDraw loop's may mark")
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered OpDraw stalled", sc.s.g.stalled)
		}
	})
	t.Run("two may replacements pose the order ask; the pick then poses its own election", func(t *testing.T) {
		// gorge asks the CR 616.1 order first, then promotes the picked
		// Optional$ line to its own apply / decline (replacement_choice.go
		// scheduleSelectedOptionalDraw); the kernel's order answer hands the
		// pick to the may election at the same site (repl_draworder.go)
		tb := newSPTable()
		a := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional))
		b := addCard(tb, mayProg(kop.ReplDrawOptional,
			kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}))
		sc := tb.scen(t)
		sc.perm(a, 0)
		hb := sc.perm(b, 0)
		hand, lib, life := sc.s.pl[0].handN, sc.s.pl[0].libLen(), sc.s.pl[0].life
		if lib == 0 || life < 2 {
			t.Fatalf("precondition: library %d, life %d before the draw", lib, life)
		}
		toDrawStep(sc)
		d := sc.expect(t, DkChoose, ChGenReplOrder)
		ob := sc.s.perms[sc.slot(hb)].obj
		pick := -1
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Source() == TCode(ob) {
				pick = i
			}
		}
		if d.N != 2 || pick < 0 {
			t.Fatalf("order ask with %d options (b's at %d), want both may lines offered", d.N, pick)
		}
		sc.s.Apply(d.Options[pick])
		drawMayAsk(t, sc, ob)
		if sc.s.ctl != ctlGenAsk || !sc.s.g.drawFirst {
			t.Fatalf("ctl %d drawFirst %v: the step was finished over the pick's election", sc.s.ctl, sc.s.g.drawFirst)
		}
		sc.s.Apply(MakeAction(AkChoose, TCode(ob), 0, 1))
		if got := sc.s.pl[0].life; got != life-1 {
			t.Fatalf("life = %d, want %d (the picked body ran on accept)", got, life-1)
		}
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the body replaced the draw)", got, hand)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered pick stalled", sc.s.g.stalled)
		}
		if d := sc.s.Step(); d.Kind != DkPriority || sc.s.g.drawFirst {
			t.Fatalf("decision kind %d (drawFirst %v): want the draw step's priority", d.Kind, sc.s.g.drawFirst)
		}
	})
	t.Run("the non-active player's draw at the draw step's start is not the turn draw", func(t *testing.T) {
		// drawOrderSite's turn arm is the active player's first draw only:
		// any other draw at that control point has no step to finish
		tb := newSPTable()
		host := addCard(tb, mayProg(kop.ReplDrawSkip|kop.ReplDrawOptional|kop.ReplDrawAnyPlayer))
		sc := tb.scen(t)
		sc.perm(host, 0)
		sc.s.step, sc.s.ctl, sc.s.g.drawFirst = kop.StepDraw, ctlStepBegin, true
		if sc.s.active != 0 {
			t.Fatalf("precondition: active seat %d, want 0", sc.s.active)
		}
		hand := sc.s.pl[1].handN
		sc.s.draw(1)
		if sc.s.res.ask == raGenDrawMay {
			t.Fatal("the non-active player's draw posed the turn draw's may election")
		}
		if sc.s.g.stalled != StallDrawRepl {
			t.Fatalf("stalled = %d, want StallDrawRepl (%d)", sc.s.g.stalled, StallDrawRepl)
		}
		if got := sc.s.pl[1].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the stalled draw was not made)", got, hand)
		}
	})
}
