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

// TestReplPlusNoncombatRaiseWalks: a raise (Ojer Axonil) beside two
// commuting additions is the mixed CR 616.1 ask walked as picked (CR 616.1e):
// the park records the running amount and the raise's candidate, the picked
// addition applies at the pick, the raise past its host's power is
// re-collected out (ncPlusLive) and the record's amount is the picked
// order's. Picking the raise first keeps the ask alive over the two
// additions, whose order is free.
func TestReplPlusNoncombatRaiseWalks(t *testing.T) {
	tbl, plus, raise := replPlusTable(true)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	sc.perm(raise, 0)
	if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), 3, 0); !ok {
		t.Fatalf("raise beside two additions: park failed (stall %s)", StallName(sc.s.StallReason()))
	}
	if ro := sc.s.g.ro; ro.n != 3 || ro.rs != 1<<2 {
		t.Fatalf("raise beside two additions: %d candidates, rs %d, want 3 and the raise's bit", ro.n, ro.rs)
	}
	if sc.s.g.nc.n != ncPend {
		t.Fatalf("raise beside two additions: parked amount %d, want the pend sentinel", sc.s.g.nc.n)
	}
	// the first addition applies at the pick (4): the raise (power 4) no
	// longer matches, so the ask ends with the second addition applied
	if sc.s.replOrderPick(MakeAction(AkChoose, 0, 0, 0)) {
		t.Fatal("raise beside two additions: the ask continues over the expired raise")
	}
	if n := sc.s.g.nc.n; n != 5 {
		t.Fatalf("raise beside two additions: picked-order amount %d, want 5 (3 + 1 + 1)", n)
	}

	// the raise picked first replaces the amount with the host's power (4)
	// and the ask continues over the two additions
	sc = newScenTable(t, tbl, Config{NoElide: true})
	src = sc.perm(v.GrizzlyBears, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	sc.perm(raise, 0)
	if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), v.GrizzlyBears, TCode(PlayerTarget1), 3, 0); !ok {
		t.Fatalf("raise picked first: park failed (stall %s)", StallName(sc.s.StallReason()))
	}
	if !sc.s.replOrderPick(MakeAction(AkChoose, 0, 0, 2)) {
		t.Fatal("raise picked first: the ask ended over the two live additions")
	}
	if n := sc.s.g.ro.an; n != 4 {
		t.Fatalf("raise picked first: running amount %d, want the host's power 4", n)
	}
	if sc.s.replOrderPick(MakeAction(AkChoose, 0, 0, 0)) {
		t.Fatal("raise picked first: the ask continues past the last addition")
	}
	if n := sc.s.g.nc.n; n != 6 {
		t.Fatalf("raise picked first: picked-order amount %d, want 6 (4 + 1 + 1)", n)
	}
}
