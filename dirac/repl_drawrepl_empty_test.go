package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDrawEmptyLibraryScan: an attempted draw from an empty library meets
// only the Draw replacements gorge's emptyDrawReplacement meets (CR 614.11): a
// mandatory body whose root is not itself a Draw, or a bodyless Prevent$ skip.
// A matching line runs and the player is not marked for the CR 704.5b loss; a
// "draw N instead" line and a body that is a Draw do not meet the attempt, so
// the mark stands exactly as before the scan moved.
func TestReplDrawEmptyLibraryScan(t *testing.T) {
	add := func(tb *spTable, c kop.CardDef) CardID {
		tb.tbl.Cards = append(tb.tbl.Cards, c)
		return CardID(len(tb.tbl.Cards) - 1)
	}
	prog := func(name string, c int16, p ...kop.Instr) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: c}, Prog: p}}}
	}
	emptyLib := func(sc *scen, p uint8) {
		sc.s.pl[p].libLo = sc.s.pl[p].libHi
	}
	t.Run("a mandatory non-Draw body applies and leaves no mark", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog("Test Empty Draw Program", 0,
			kop.Instr{Op: kop.OpGainLife, Obj: kop.RefTriggerPlayer, A: kop.Const(2)}))
		sc := tb.scen(t)
		sc.perm(host, 0)
		emptyLib(sc, 0)
		life, hand := sc.s.pl[0].life, sc.s.pl[0].handN
		sc.s.draw(0)
		if got := sc.s.pl[0].life; got != life+2 {
			t.Fatalf("life = %d, want %d (the program ran in the draw's place)", got, life+2)
		}
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the draw was replaced)", got, hand)
		}
		if sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatal("the replaced empty-library draw still marked the player for the CR 704.5b loss")
		}
	})
	t.Run("a bodyless Prevent$ skip applies and leaves no mark", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog("Test Empty Draw Skip", kop.ReplDrawSkip))
		sc := tb.scen(t)
		sc.perm(host, 0)
		emptyLib(sc, 0)
		hand := sc.s.pl[0].handN
		sc.s.draw(0)
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (the draw was skipped)", got, hand)
		}
		if sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatal("the skipped empty-library draw still marked the player for the CR 704.5b loss")
		}
	})
	t.Run("a draw N instead line does not meet an empty-library draw", func(t *testing.T) {
		tb := newSPTable()
		// A program on seat 1 keeps the program scan on but admits only its
		// own controller's draws, so seat 0's draw meets only ReplDrawMore.
		other := add(tb, prog("Test Other Program", 0,
			kop.Instr{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}))
		more := add(tb, kop.CardDef{Name: "Test Draw More", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
		sc := tb.scen(t)
		sc.perm(other, 1)
		sc.perm(more, 0)
		if !sc.s.drawReplace(0, drawChain{}, false) {
			t.Fatal("control: the draw N line was not seen with a non-empty library")
		}
		emptyLib(sc, 0)
		hand := sc.s.pl[0].handN
		if sc.s.drawReplace(0, drawChain{}, true) {
			t.Fatal("the scan applied a draw N instead line to an empty-library attempt")
		}
		sc.s.draw(0)
		if got := sc.s.pl[0].handN; got != hand {
			t.Fatalf("hand = %d, want %d (a draw N body must not draw from the empty library)", got, hand)
		}
		if sc.s.pl[0].mark&pmDrewEmpty == 0 {
			t.Fatal("no mark: a draw N instead line must not meet an empty-library draw")
		}
	})
	t.Run("a program whose body is itself a Draw does not meet the attempt", func(t *testing.T) {
		tb := newSPTable()
		host := add(tb, prog("Test Draw Body", kop.ReplDrawBodyDraw,
			kop.Instr{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)},
			kop.Instr{Op: kop.OpGainLife, Obj: kop.RefTriggerPlayer, A: kop.Const(3)}))
		sc := tb.scen(t)
		sc.perm(host, 0)
		emptyLib(sc, 0)
		life := sc.s.pl[0].life
		sc.s.draw(0)
		if got := sc.s.pl[0].life; got != life {
			t.Fatalf("life = %d, want %d (the Draw-rooted body must not run on an empty library)", got, life)
		}
		if sc.s.pl[0].mark&pmDrewEmpty == 0 {
			t.Fatal("no mark: a Draw-rooted body is excluded from the empty-library scan (gorge emptyDrawReplacement)")
		}
	})
}
