package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// edhSet is the edh table (MTGJSON Commander precons gorge fully supports, with
// the kernel-refused copies replaced by basics) in the new-pool mode, kfuzz
// -allow-unverified: no edh card has its own evidence yet. It skips without the
// corpus or the tables.
func edhSet(t *testing.T) cardset.Set {
	t.Helper()
	if _, err := PoolByName("edh"); err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return cardset.AllowUnverified(set)
}

// TestEDHPoolHoldsTheSmokeCommanders: kfuzz -kernel mbc on the Bloomburrow
// Commander precon pairs (design 2026-11-17 xengine Commander diffs, section 8)
// refused every pair with `card "Bello, Bard of the Brambles" is not in the
// pool`: no registered pool held a precon. The edh pool's table holds the three
// BLC precons gorge fully supports, so a game holding a commander either builds
// its table or is refused by name for a lowering the kernel lacks (a
// RefusedError, counted as refused-card), never "not in the pool".
func TestEDHPoolHoldsTheSmokeCommanders(t *testing.T) {
	set := edhSet(t)
	basics := []cardset.Entry{{Name: "Forest", Count: 99}}
	for _, cmd := range []string{"Bello, Bard of the Brambles", "Ms. Bumbleflower", "Hazel of the Rootbloom"} {
		deck := []cardset.Entry{{Name: cmd, Count: 1}, {Name: "Forest", Count: 99}}
		_, _, err := set.MatchupN([][]cardset.Entry{deck, append([]cardset.Entry{{Name: "Plains", Count: 1}}, basics...)})
		var re *cardset.RefusedError
		switch {
		case err == nil:
			t.Logf("%s: plays", cmd)
		case errors.As(err, &re):
			t.Logf("%s: refused by the kernel (%s): %s", cmd, re.Status, re.Reason)
		default:
			t.Errorf("%s: %v, want a table or a refusal by name", cmd, err)
		}
	}
}

// TestEDHPairTables builds the kernel table of every pair of the committed edh
// slice (kfuzz -kernel edh, 1v1 Commander). Each deck holds 100 cards, so a duel
// is 200 card instances inside dirac.MaxObj, the rest of which is token
// headroom (a game that needs more is a capacity stall, counted by kfuzz); the
// pair's subtypes fit kop.MaxSubtypes. A pair the kernel refuses for a card
// interaction or a card refused by name since the slice was cut is counted; any
// other error fails.
func TestEDHPairTables(t *testing.T) {
	set := edhSet(t)
	base, _ := testCorpus(t)
	dir, remove, err := ExtractSlice("edh")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	lines, err := ExplicitPairs(filepath.Join(dir, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	entries := map[string][]cardset.Entry{}
	deck := func(n string) []cardset.Entry {
		if es, ok := entries[n]; ok {
			return es
		}
		f, err := c.deckFile(n)
		if err != nil {
			t.Fatal(err)
		}
		var es []cardset.Entry
		size := 0
		for _, e := range f.Cards {
			es = append(es, cardset.Entry{Name: e.Name, Count: e.Count})
			size += e.Count
		}
		if size != 100 {
			t.Fatalf("deck %s holds %d cards, want 100", n, size)
		}
		entries[n] = es
		return es
	}
	if 200 > dirac.MaxObj {
		t.Fatalf("a 1v1 Commander duel is 200 card instances, past dirac.MaxObj %d", dirac.MaxObj)
	}
	built, refused, most := 0, 0, 0
	reasons := map[string]int{}
	for i, l := range lines {
		tbl, _, err := set.MatchupN([][]cardset.Entry{deck(l.A), deck(l.B)})
		if why := stdMetaPairRefusal(err); why != "" {
			refused++
			reasons[why]++
			continue
		}
		var re *cardset.RefusedError
		if errors.As(err, &re) {
			refused++
			reasons["refused "+re.Card]++
			continue
		}
		if err != nil {
			t.Fatalf("pair %d %s vs %s: %v", i, l.A, l.B, err)
		}
		if len(tbl.Subtypes) > kop.MaxSubtypes {
			t.Fatalf("pair %d: %d subtypes, past kop.MaxSubtypes %d", i, len(tbl.Subtypes), kop.MaxSubtypes)
		}
		built++
		most = max(most, len(tbl.Subtypes))
	}
	t.Logf("edh slice: %d pairs, %d tables built (most subtypes %d), %d refused %v; token headroom %d objects",
		len(lines), built, most, refused, reasons, dirac.MaxObj-200)
	if built == 0 {
		t.Fatalf("no edh pair built a table: %v", reasons)
	}
}
