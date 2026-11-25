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
		if sc.k.gen == nil || !sc.k.gen.anyDrawProgram {
			t.Fatal("precondition: the gen flag anyDrawProgram is not set")
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
}
