package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ReplDrawProgram draw pipeline (dirac/repl_drawprogram.go): a Draw
// replacement's ReplaceWith$ body is a program run in the draw's place, the
// draw's player is ReplacedPlayer, the scan reaches every seat's battlefield
// for ValidPlayer$ Opponent / Player, a bodyless Prevent$ skips the draw, and
// the body's own draws do not meet the replacement that is replacing them
// (CR 614.5).
func TestReplDrawProgramScenarios(t *testing.T) {
	prog := func(kind kop.ReplKind, c int16, p ...kop.Instr) kop.CardDef {
		return kop.CardDef{Name: "Test Draw Replacement", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kind, C: c}, Prog: p}}}
	}
	add := func(tb *spTable, c kop.CardDef) CardID {
		tb.tbl.Cards = append(tb.tbl.Cards, c)
		return CardID(len(tb.tbl.Cards) - 1)
	}
	t.Run("the body's draws and life loss replace the draw", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog(kop.ReplDrawProgram, 0,
			kop.Instr{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(2)},
			kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}))
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		if !sc.onBF(h) {
			t.Fatal("precondition: the replacement host is not on the battlefield")
		}
		if sc.k.gen == nil || !sc.k.gen.hasDrawProgram() {
			t.Fatal("precondition: the gen flag hasDrawProgram is not set")
		}
		hand, life := sc.s.pl[0].handN, sc.s.pl[0].life
		sc.s.draw(0)
		if got := sc.s.pl[0].handN; got != hand+2 {
			t.Fatalf("hand = %d, want %d (the body drew two in the draw's place)", got, hand+2)
		}
		if got := sc.s.pl[0].life; got != life-1 {
			t.Fatalf("life = %d, want %d (the body lost one)", got, life-1)
		}
	})
	t.Run("an opponent's replacement draws for its controller", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog(kop.ReplDrawProgram, kop.ReplDrawOpponent,
			kop.Instr{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}))
		sc := tb.scen(t)
		if !sc.onBF(sc.perm(host, 1)) {
			t.Fatal("precondition: the replacement host is not on seat 1")
		}
		drawerHand, ctrlHand := sc.s.pl[0].handN, sc.s.pl[1].handN
		sc.s.draw(0)
		if got := sc.s.pl[0].handN; got != drawerHand {
			t.Fatalf("drawer hand = %d, want %d (the draw was replaced)", got, drawerHand)
		}
		if got := sc.s.pl[1].handN; got != ctrlHand+1 {
			t.Fatalf("controller hand = %d, want %d (Notion Thief's controller drew)", got, ctrlHand+1)
		}
	})
	t.Run("an any-player replacement matches either seat's draw", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog(kop.ReplDrawProgram, kop.ReplDrawAnyPlayer|kop.ReplDrawSkip))
		sc := tb.scen(t)
		sc.perm(host, 1)
		for _, p := range []uint8{0, 1} {
			hand := sc.s.pl[p].handN
			sc.s.draw(p)
			if got := sc.s.pl[p].handN; got != hand {
				t.Fatalf("seat %d drew with Possessed Portal out: hand = %d, want %d", p, got, hand)
			}
		}
	})
	t.Run("NotFirstCardInDrawStep exempts the first draw of the draw step", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog(kop.ReplDrawProgram, kop.ReplDrawOpponent|kop.ReplDrawNotFirst|kop.ReplDrawSkip))
		sc := tb.scen(t)
		sc.perm(host, 1)
		sc.s.step, sc.s.active, sc.s.g.drawFirst = kop.StepDraw, 0, true
		drawerHand, ctrlHand := sc.s.pl[0].handN, sc.s.pl[1].handN
		sc.s.draw(0)
		if got := sc.s.pl[0].handN; got != drawerHand+1 {
			t.Fatalf("first draw of the draw step: drawer hand = %d, want %d (exempt)", got, drawerHand+1)
		}
		if got := sc.s.pl[1].handN; got != ctrlHand {
			t.Fatalf("the exempt draw was replaced: controller hand = %d, want %d", got, ctrlHand)
		}
	})
	t.Run("a Dig body runs in the draw's place", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog(kop.ReplDrawProgram, 0,
			kop.Instr{Op: kop.OpDigEx, Obj: kop.RefTriggerPlayer, A: kop.Const(3), B: kop.Const(-1),
				C: int16(kop.ZoneHand) | int16(kop.ZoneLibrary)<<3, F: 0}))
		sc := tb.scen(t)
		sc.perm(host, 0)
		hand, lib := sc.s.pl[0].handN, sc.s.pl[0].libLen()
		sc.s.draw(0)
		if got := sc.s.pl[0].libLen(); got >= lib {
			t.Fatalf("library = %d, want fewer than %d (the dig looked at three)", got, lib)
		}
		if got := sc.s.pl[0].handN; got <= hand {
			t.Fatalf("hand = %d, want more than %d (the dig took cards)", got, hand)
		}
	})
	t.Run("a second matching replacement stalls rather than pick an order", func(t *testing.T) {
		tb := newSPTable()
		a := add(tb, prog(kop.ReplDrawProgram, kop.ReplDrawAnyPlayer|kop.ReplDrawSkip))
		b := add(tb, prog(kop.ReplDrawProgram, kop.ReplDrawAnyPlayer|kop.ReplDrawSkip))
		sc := tb.scen(t)
		sc.perm(a, 0)
		sc.perm(b, 1)
		sc.s.draw(0)
		if sc.s.g.stalled != StallDrawRepl {
			t.Fatalf("stalled = %d, want StallDrawRepl (%d)", sc.s.g.stalled, StallDrawRepl)
		}
	})
	t.Run("two ReplDrawMore compose instead of stalling", func(t *testing.T) {
		// The table needs a ReplDrawProgram for drawReplace to scan, but it
		// belongs to seat 1 and admits only its controller's draws, so seat
		// 0's draw meets only the two commutative ReplDrawMore lines.
		tb := newSPTable()
		p := add(tb, prog(kop.ReplDrawProgram, 0,
			kop.Instr{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}))
		more := func(n int16) CardID {
			return add(tb, kop.CardDef{Name: "Draw More", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: n}}}})
		}
		a, b := more(2), more(3)
		sc := tb.scen(t)
		sc.perm(p, 1)
		sc.perm(a, 0)
		sc.perm(b, 0)
		hand := sc.s.pl[0].handN
		sc.s.draw(0)
		if sc.s.g.stalled == StallDrawRepl {
			t.Fatal("two ReplDrawMore lines stalled; drawMore composes them in scan order (repl_draw.go)")
		}
		if got, want := int(sc.s.pl[0].handN), int(hand)+2*3; got != want {
			t.Fatalf("hand = %d, want %d (2 draws x 3 cards)", got, want)
		}
	})
}

// TestReplDrawProgramKeepsOuterPC: a draw replacement body runs inside the
// outer resolution when a resolving spell's draw is replaced. The body must
// not leave its own pc (or any other resCtx field) behind: gorge resumes the
// spell at the instruction after its Draw.
func TestReplDrawProgramKeepsOuterPC(t *testing.T) {
	// cast runs a spell whose program is prog with the host's draw replacement
	// on the battlefield, through the same resolver runProg a real spell uses.
	cast := func(t *testing.T, body []kop.Instr, prog []kop.Instr) *scen {
		t.Helper()
		tb := newSPTable()
		hostID := CardID(len(tb.tbl.Cards))
		tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Replacement", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram}, Prog: body}}})
		spell := CardID(len(tb.tbl.Cards))
		tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Spell", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
		sc := tb.scen(t)
		host := sc.perm(hostID, 0) // the replacement permanent
		s := sc.s
		_ = host
		s.sp = 1
		s.res = resCtx{e: stackEntry{kind: seAbility, obj: s.perms[sc.slot(host)].obj,
			srcVer: s.objs[s.perms[sc.slot(host)].obj].ver, card: spell, ab: 0, ctrl: 0},
			active: true, lastTok: NoObj, eachObj: NoObj}
		s.runProg(sc.k)
		return sc
	}
	t.Run("a two-op body does not skip the spell's next instruction", func(t *testing.T) {
		sc := cast(t,
			[]kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(2)},
				{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)},
			},
			[]kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)},
			})
		if got, want := int(sc.s.pl[0].handN), 2; got != want {
			t.Fatalf("hand = %d, want %d (the body drew two in the draw's place)", got, want)
		}
		if got, want := int(sc.s.pl[0].life), 20-1+5; got != want {
			t.Fatalf("life = %d, want %d (the body lost one and the spell's GainLife ran)", got, want)
		}
	})
	t.Run("a one-op body does not rewind the spell to its first instruction", func(t *testing.T) {
		// The body sets the borrowed res.pc to 0. Without the restore runProg
		// resumes the spell at pc 1 -- its Draw -- and replaces it again, so
		// this once looped forever (the reviewer's Notion Thief shape).
		sc := cast(t,
			[]kop.Instr{{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}},
			[]kop.Instr{
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)},
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)},
			})
		if got, want := int(sc.s.pl[0].handN), 1; got != want {
			t.Fatalf("hand = %d, want %d (the body drew one and the spell did not rewind)", got, want)
		}
		if got, want := int(sc.s.pl[0].life), 20+1+5; got != want {
			t.Fatalf("life = %d, want %d (the spell's two GainLife instructions ran once each)", got, want)
		}
	})
}
