package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDrawOrderRememberDrawnStalls: an OpDraw carrying RememberDrawn$
// draws through drawRemember (zone_rememberwrites.go), which polls the
// drawer's hand after the draw call returns. A CR 616.1 order ask suspends
// the draw before the picked replacement body runs, so the poll sees none of
// the resumed body's cards. That would be a silent divergence from gorge's
// recordDrawn (which records after the replacement chain completes), so the
// shape keeps the declared StallDrawRepl.
func TestReplDrawOrderRememberDrawnStalls(t *testing.T) {
	for _, all := range []bool{false, true} {
		name := "RememberDrawn"
		if all {
			name = "RememberDrawnAllReplaced"
		}
		t.Run(name, func(t *testing.T) {
			tb := newSPTable()
			more := CardID(len(tb.tbl.Cards))
			tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw More", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
			prog := CardID(len(tb.tbl.Cards))
			tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Draw Program", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer},
					Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}}}}})
			flags := int16(kop.MvRemember)
			if all {
				flags |= kop.DrawRememberAll
			}
			spell := CardID(len(tb.tbl.Cards))
			tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Remember Draw", Types: kop.TSorcery,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
					{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1), C: flags}}}}})
			sc := tb.scen(t)
			plainObj := sc.perm(more, 0)
			progObj := sc.perm(prog, 1)
			if !sc.onBF(plainObj) || !sc.onBF(progObj) {
				t.Fatal("precondition: both draw replacements must be on the battlefield")
			}
			if got := sc.s.drawScan(sc.k, 0, drawChain{}); got.more != 1 || !got.prog || got.all != 2 {
				t.Fatalf("precondition: candidates more %d prog %v all %d, want one plain ReplDrawMore beside the program", got.more, got.prog, got.all)
			}
			if in := tb.tbl.Cards[spell].Abilities[0].Prog[0]; in.Op != kop.OpDraw || in.C&kop.MvRemember == 0 {
				t.Fatalf("precondition: the resolving instruction is op %d C %d, want the OpDraw carrying RememberDrawn$", in.Op, in.C)
			}
			s := sc.s
			drawOrderResSpell(sc, spell, sc.obj(prog, 1))
			if s.res.ask == raGenDrawOrder {
				// Show the divergence the stall avoids: the picked program's body
				// draws two through the plain replacement, but drawRemember's
				// hand poll already ran, so the remembered set stays empty.
				d := drawOrderAsk(t, sc)
				picked := -1
				for i := 0; i < int(d.N); i++ {
					if d.Options[i].Source() == TCode(progObj) {
						picked = i
					}
				}
				if picked < 0 {
					t.Fatalf("the program is not among the %d order options", d.N)
				}
				hand := s.pl[0].handN
				s.applyResolveGen(sc.k, d.Options[picked])
				t.Fatalf("the RememberDrawn$ draw suspended on the order ask: hand +%d, remembered %d; the hand poll ran before the body, so this shape must declare StallDrawRepl %d",
					s.pl[0].handN-hand, s.g.nrem, StallDrawRepl)
			}
			if s.g.stalled != StallDrawRepl {
				t.Fatalf("stalled = %d, want StallDrawRepl %d", s.g.stalled, StallDrawRepl)
			}
		})
	}
}
