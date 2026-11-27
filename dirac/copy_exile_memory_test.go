package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileMemoryCounterCopy: Altair Ibn-La'Ahad's ChangeZone exiles its target
// with a MEMORY counter (CR 122.1) and the CopyAllExile sweep copies each of
// the controller's exiled creature cards with one. The kernel records MEMORY in
// the exile-counter table, so the sweep's counters>=1 MEMORY filter finds the
// marked card and skips an unmarked one. Without the record the sweep matched
// nothing and dirac minted no copy where gorge does.
func TestExileMemoryCounterCopy(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	exileMem := kop.Instr{Op: kop.OpExileEx, Obj: kop.RefTarget0, A: kop.Const(1), B: kop.Const(int(kop.CtrMemory))}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Assassin", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Prog: []kop.Instr{exileMem}}}},
		kop.CardDef{Name: "T Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneExile,
		Pred: kop.PredCountersGE, PredN: int16(kop.CtrMemory) | 1<<8})
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.uses.has(useExileCtr) {
		t.Fatal("a table exiling with a MEMORY counter has no useExileCtr bit")
	}

	marked := sc.obj(n, 0)
	plain := sc.obj(n+1, 0)
	sc.s.place(marked, kop.ZoneExile, false)
	sc.s.exileMoveCounter(sc.k, &exileMem, marked)
	sc.s.place(plain, kop.ZoneExile, false)
	if got := sc.s.exileCounters(marked, kop.CtrMemory); got != 1 {
		t.Fatalf("the marked card carries %d MEMORY counters in exile, want 1", got)
	}
	if got := sc.s.exileCounters(plain, kop.CtrMemory); got != 0 {
		t.Fatalf("the unmarked card carries %d MEMORY counters, want 0", got)
	}

	src := sc.perm(n+1, 0)
	in := kop.ExtInstr(kop.ExtCopyExiledWith)
	in.F = f
	in.B = kop.Const(kop.CopyNonRemCtrl)
	sc.s.res.active, sc.s.res.e.obj, sc.s.res.e.ctrl = true, src, 0
	sc.s.copyExiledWith(sc.k, &in)
	if got := copyExwTokens(sc.s); got != 1 || sc.s.g.stalled != 0 {
		t.Fatalf("copies %d stalled %d, want one copy (the marked card) and no stall", got, sc.s.g.stalled)
	}
}
