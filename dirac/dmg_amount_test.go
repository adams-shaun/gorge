package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDmgAmountRecord: Tangled Colony's Count$TotalDamageReceivedThisTurn
// reads the positive damage it received this turn (two hits of 2 and 3 read
// 5), the record clears at the turn change, and a fifth reader-card object
// damaged in one turn stalls by name.
func TestDmgAmountRecord(t *testing.T) {
	set := cardsettest.Synth(t, "std-woe")
	deck := []cardset.Entry{{Name: "Tangled Colony", Count: 5}, {Name: "Plains", Count: 12}}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-woe table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	tc := id["Tangled Colony"]
	// the card is the table's reader, and the table opened the sites
	if !sc.k.t.Cards[tc].DmgAmt || !sc.k.gen.anyDmgAmt {
		t.Fatalf("Tangled Colony: CardDef.DmgAmt %v, genPrep.anyDmgAmt %v", sc.k.t.Cards[tc].DmgAmt, sc.k.gen.anyDmgAmt)
	}
	idx := int16(-1)
	for i := range sc.k.t.Exprs {
		if sc.k.t.Exprs[i].Kind == kop.ExDmgAmountReceived {
			idx = int16(i)
			break
		}
	}
	if idx < 0 {
		t.Fatal("no ExDmgAmountReceived expression in the table")
	}
	o := sc.perm(tc, 0)
	sl := sc.slot(o)
	if sc.s.dmgAmountReceived(o) != 0 {
		t.Fatal("a fresh object already holds damage")
	}
	sc.s.dmgPerm(sc.k, sl, &sc.s.perms[sl], 2)
	sc.s.dmgPerm(sc.k, sl, &sc.s.perms[sl], 3)
	if got := sc.s.expr(sc.k, idx, evalCtx{src: o}); got != 5 {
		t.Fatalf("two hits of 2 and 3 read %d, want 5", got)
	}

	sc.s.newTurnGen()
	if got := sc.s.expr(sc.k, idx, evalCtx{src: o}); got != 0 {
		t.Fatalf("the record survived the turn change: %d", got)
	}

	objs := []Obj{o}
	for i := 0; i < dmgAmtN; i++ {
		objs = append(objs, sc.perm(tc, 1))
	}
	for _, x := range objs {
		sl := sc.slot(x)
		sc.s.dmgPerm(sc.k, sl, &sc.s.perms[sl], 1)
	}
	if got := StallName(sc.s.g.stalled); got != "dmgamt" {
		t.Fatalf("a fifth damaged reader stalled with %q, want dmgamt", got)
	}
}
