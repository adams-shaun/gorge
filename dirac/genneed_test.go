package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// withRegistries runs f with the prep registries holding only what f
// registers, then restores them.
func withRegistries(t *testing.T, f func()) {
	t.Helper()
	needs, uses := genNeeds, genUses
	genNeeds, genUses = nil, nil
	defer func() { genNeeds, genUses = needs, uses }()
	f()
}

// TestPrepGenNeedOpensTheGate: a table prepareGen's own gate leaves without a
// genPrep gets one when a registered need reports it, and only then.
func TestPrepGenNeedOpensTheGate(t *testing.T) {
	plain := arenaTable(kop.OpRegenerate)
	if prepare(plain).gen != nil {
		t.Fatal("the plain table already has a genPrep: pick one the gate leaves")
	}
	withRegistries(t, func() {
		genNeed(func(tb *kop.Table) bool { return len(tb.Cards) == 2 })
		if prepare(plain).gen != nil {
			t.Error("a need that does not hold opened the gate")
		}
		two := arenaTable(kop.OpRegenerate)
		two.Cards = append(two.Cards, kop.CardDef{Name: "U"})
		if prepare(two).gen == nil {
			t.Error("a need that holds left the table without a genPrep")
		}
	})
}

// TestPrepGenUseSetsItsBit: each registered use's bit is set exactly for the
// tables its detector reports, and a use that holds opens the gate.
func TestPrepGenUseSetsItsBit(t *testing.T) {
	withRegistries(t, func() {
		never := newGenUse(func(*kop.Table) bool { return false })
		regen := newGenUse(func(tb *kop.Table) bool { return tb.Cards[0].Abilities[0].Prog[0].Op == kop.OpRegenerate })
		if never == regen {
			t.Fatal("two uses share a bit")
		}
		g := prepare(arenaTable(kop.OpRegenerate)).gen
		if g == nil {
			t.Fatal("a use that holds left the table without a genPrep")
		}
		if !g.uses.has(regen) || g.uses.has(never) {
			t.Errorf("uses %v: want regen's bit %d alone", g.uses, regen)
		}
		if g := prepare(arenaTable(kop.OpGoad)).gen; g != nil {
			t.Errorf("no use holds, yet the table has a genPrep (uses %v)", g.uses)
		}
	})
}

// TestPrepGenUseBitsSpanWords: uses past the first word land in later words,
// and the registry refuses one past useWords*64.
func TestPrepGenUseBitsSpanWords(t *testing.T) {
	withRegistries(t, func() {
		var last genUse
		for i := 0; i < useWords*64; i++ {
			last = newGenUse(func(*kop.Table) bool { return true })
		}
		s := tableUses(arenaTable())
		for i := 0; i < useWords*64; i++ {
			if !s.has(genUse(i)) {
				t.Fatalf("bit %d unset", i)
			}
		}
		if last != useWords*64-1 {
			t.Fatalf("last use %d", last)
		}
		defer func() {
			if recover() == nil {
				t.Error("a use past useWords*64 registered")
			}
		}()
		newGenUse(func(*kop.Table) bool { return true })
	})
}
