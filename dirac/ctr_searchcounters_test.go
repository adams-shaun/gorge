package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrepEntryMergeSearchCounterArm pins the source test for the entry-merge
// window's library-search arm: only a POSITIVE OpExt SearchCounters arm of a
// merging kind is a source. kgen emits the trailing clear as
// `SearchCounters B=0` with C=0, and CtrP1P1 is 0, so a kind-only test would
// enable the window (and reorder s.g.enterCtr against pc.enterCtr) for every
// table with a search rider, including CHARGE/STUN ones the window never
// merges.
func TestPrepEntryMergeSearchCounterArm(t *testing.T) {
	table := func(instrs ...kop.Instr) *kop.Table {
		return &kop.Table{Cards: []kop.CardDef{{Abilities: []kop.Ability{{Prog: instrs}}}}}
	}
	ext := func(amount int, kind kop.CounterKind) kop.Instr {
		in := kop.ExtInstr(kop.ExtSearchCounters)
		in.B = kop.Const(amount)
		in.C = int16(kind)
		return in
	}

	g := &genPrep{anyRepl: rbDblCtr}
	prepEntryMerge(table(ext(0, kop.CtrP1P1)), g)
	if g.anyEntryMerge {
		t.Fatal("the trailing B=0 SearchCounters clear enabled the entry-merge window")
	}

	g = &genPrep{anyRepl: rbDblCtr}
	prepEntryMerge(table(ext(1, kop.CtrP1P1)), g)
	if !g.anyEntryMerge {
		t.Fatal("a +1/+1 SearchCounters arm did not enable the entry-merge window")
	}

	g = &genPrep{anyRepl: rbDblCtr}
	prepEntryMerge(table(ext(7, kop.CtrCharge)), g)
	if g.anyEntryMerge {
		t.Fatal("a CHARGE SearchCounters arm enabled the entry-merge window")
	}
}
