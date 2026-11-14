package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGraveExilePickMutation: the dual-origin walk reads the Exile half. The
// constructed graveexile pool has no supported mill-to-exile effect (Leyline of the
// Void and Rest in Peace are repl-hard), so no full-game lockstep puts a
// remembered milled card in exile; this focused test seeds one and requires
// the hidden pick to offer it, then flips DebugIgnoreGraveExile and requires
// the offer gone.
func TestGraveExilePickMutation(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	o := sc.obj(v.Plains, 0)
	sc.s.place(o, kop.ZoneExile, false)
	sc.s.remember(o)
	sc.s.res.e = stackEntry{ctrl: 0}
	in := &kop.Instr{Op: kop.OpChooseMove}

	var d Decision
	sc.s.graveExileCands(sc.k, in, &d)
	found := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(o)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("exile pick did not offer the remembered exiled card: %v", d.Options[:d.N])
	}

	old := DebugIgnoreGraveExile
	DebugIgnoreGraveExile = true
	defer func() { DebugIgnoreGraveExile = old }()
	d = Decision{}
	sc.s.graveExileCands(sc.k, in, &d)
	if d.N != 0 {
		t.Fatalf("DebugIgnoreGraveExile still offered %d options", d.N)
	}
}

// TestGraveExileChangeZoneAllMutation: a Graveyard,Exile ChangeZoneAll
// (OpChangeZoneAllEx + MvGraveExile) moves an exiled card to the hand, and
// DebugIgnoreGraveExile (the dropped Exile half) leaves it in exile.
func TestGraveExileChangeZoneAllMutation(t *testing.T) {
	run := func(ignore bool) kop.Zone {
		old := DebugIgnoreGraveExile
		DebugIgnoreGraveExile = ignore
		defer func() { DebugIgnoreGraveExile = old }()
		tbl := castMaypermissionLimitTable() // a table whose prep arms k.gen
		tbl.Filters = append(tbl.Filters, kop.Filter{})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		o := sc.obj(v.Plains, 0)
		sc.s.place(o, kop.ZoneExile, false)
		sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0}
		in := kop.Instr{Op: kop.OpChangeZoneAllEx, Obj: kop.RefNone, F: kop.FilterID(len(tbl.Filters) - 1),
			C: int16(kop.ZoneHand) | kop.MvGraveExile<<3}
		sc.s.execChangeZoneAll(sc.k, &in, false)
		return sc.s.objs[o].zone
	}
	if z := run(false); z != kop.ZoneHand {
		t.Fatalf("exiled card ended in zone %v, want the hand", z)
	}
	if z := run(true); z != kop.ZoneExile {
		t.Fatalf("DebugIgnoreGraveExile still moved the exiled card (zone %v)", z)
	}
}
