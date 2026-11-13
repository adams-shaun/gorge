package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// replPlusTable appends a "source you control" filter, one additive
// ReplDamagePlus card and, when withRaise, Ojer Axonil's raise body (a
// ReplDmgRaise replacement whose power is the raised amount) to vanilla.
func replPlusTable(withRaise bool) (*kop.Table, CardID, CardID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	plus := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Plus", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind: kop.ReplDamagePlus, A: 1,
			C:      int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm),
			Filter: fid}}}})
	raise := plus
	if withRaise {
		raise = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Raise", Types: kop.TEnchantment, Power: 4,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
				Kind:   kop.ReplDamagePlus,
				C:      int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm | kop.ReplDmgRaise),
				Filter: fid}}}})
	}
	return &tbl, plus, raise
}

// TestReplPlusNoncombatPark: two commuting plus-N replacements matching one
// noncombat damage event park gorge's CR 616.1 order ask with the sum already
// known, so the park poses the same two options and deals n + the additions.
func TestReplPlusNoncombatPark(t *testing.T) {
	tbl, plus, _ := replPlusTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), 3, 0); !ok {
		t.Fatalf("two commuting plus-N replacements: park failed (stall %s)", StallName(sc.s.StallReason()))
	}
	if sc.s.Stalled() {
		t.Fatalf("two commuting plus-N replacements: stalled %s", StallName(sc.s.StallReason()))
	}
	if n := sc.s.g.ro.n; n != 2 {
		t.Fatalf("two commuting plus-N replacements: %d candidates, want 2", n)
	}
	if n := sc.s.g.nc.n; n != 5 {
		t.Fatalf("two commuting plus-N replacements: parked amount %d, want 5 (3 + 1 + 1)", n)
	}
}

// TestReplPlusNoncombatRaiseStalls: a raise (Ojer Axonil) beside two commuting
// additions is the non-commuting CR 616.1 ask this park cannot sum. The park
// must leave the event to replDamage, whose replPlus caps StallReplPlus
// loudly instead of parking only the additions and dropping the raise.
func TestReplPlusNoncombatRaiseStalls(t *testing.T) {
	tbl, plus, raise := replPlusTable(true)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	sc.perm(raise, 0)
	if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), 3, 0); ok {
		t.Fatalf("raise beside two additions parked %d candidates for %d damage, dropping the raise",
			sc.s.g.ro.n, sc.s.g.nc.n)
	}
	if sc.s.Stalled() {
		t.Fatalf("raise beside two additions stalled in the park, want replDamage's stall: %s", StallName(sc.s.StallReason()))
	}
	if n := sc.s.g.nc.n; n != 0 {
		t.Fatalf("raise beside two additions left a parked event of %d, want none", n)
	}
	if got := sc.s.replDamage(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), false, 3); got != 4 {
		t.Fatalf("raise beside two additions: replDamage %d, want 4", got)
	}
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("raise beside two additions: stall %q, want replplus", StallName(got))
	}
}
