package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReplPlusNoncombatCrossSeat: a plus-N line need not belong to the
// source's controller (CR 616.1; the recipient's Tok-Tok, Volcano Born), so
// the noncombat park collects it from the recipient's battlefield too. Seat
// 0's Grizzly Bears deals 3 to seat 1: seat 0's plus line ("a source you
// control") matches beside seat 1's ("a source an opponent controls"), and
// the parked amount is the sum. parkPlusCand re-matches with the host's own
// controller, so a seat-1 line that does not match the source (one reading
// "a source you control" of seat 1) is not collected.
func TestReplPlusNoncombatCrossSeat(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	you := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	opp := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlOpp})
	plus0 := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Plus You", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind: kop.ReplDamagePlus, A: 1,
			C:      int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm),
			Filter: you}}}})
	plus1 := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Plus Opp", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind: kop.ReplDamagePlus, A: 1,
			C:      int16(kop.ReplDmgPlayer | kop.ReplDmgPerm),
			Filter: opp}}}})
	plusMis := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Plus Unmatched", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind: kop.ReplDamagePlus, A: 1,
			C:      int16(kop.ReplDmgPlayer | kop.ReplDmgPerm),
			Filter: you}}}})

	sc := newScenTable(t, &tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(plus0, 0)
	sc.perm(plus1, 1)
	sc.perm(plusMis, 1)
	if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), 3, 0); !ok {
		t.Fatalf("the recipient's plus beside the source controller's: park failed (stall %s)", StallName(sc.s.StallReason()))
	}
	if sc.s.Stalled() {
		t.Fatalf("the recipient's plus beside the source controller's: stalled %s", StallName(sc.s.StallReason()))
	}
	if n := sc.s.g.ro.n; n != 2 {
		t.Fatalf("the recipient's plus beside the source controller's: %d candidates, want 2 (the unmatched line not collected)", n)
	}
	if n := sc.s.g.nc.n; n != 5 {
		t.Fatalf("the recipient's plus beside the source controller's: parked amount %d, want 5 (3 + 1 + 1)", n)
	}
}
