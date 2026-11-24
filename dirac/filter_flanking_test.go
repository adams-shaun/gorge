package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWithoutFlankingWord pins kop.W7WithoutFlanking (gorge's withoutFlanking,
// CR 702.25a's blocker without flanking) on a battlefield permanent and on a
// card off it: a printed flanking creature fails the word, a plain one
// passes, and a flanking permanent that lost all abilities passes.
func TestWithoutFlankingWord(t *testing.T) {
	set := cardsettest.Set(t, "flanking")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Pred: kop.PredWord7, PredN: kop.W7WithoutFlanking})
	f := kop.FilterID(len(tbl.Filters) - 1)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	cav, bears := id("Benalish Cavalry"), id("Grizzly Bears")
	sc := newScenTable(t, &tbl, Config{})
	s, k := sc.s, sc.k
	if !k.t.Cards[cav].Flanking || k.t.Cards[bears].Flanking {
		t.Fatal("Benalish Cavalry must print flanking and Grizzly Bears must not")
	}
	fl, pl := sc.perm(cav, 0), sc.perm(bears, 0)
	onBF := func(o Obj) bool { return s.matchPermGen(k, f, sc.slot(o), 0, NoSlot, 0) }
	if onBF(fl) || !onBF(pl) {
		t.Fatalf("battlefield: flanking passes %v (want false), plain passes %v (want true)", onBF(fl), onBF(pl))
	}
	inHand := func(c CardID, o Obj) bool { return s.matchCardGen(k, f, c, 0, 0, o) }
	if inHand(cav, sc.hand(cav, 0)) || !inHand(bears, sc.hand(bears, 0)) {
		t.Fatal("off the battlefield the printed card decides")
	}
	s.perms[sc.slot(fl)].ovr.flags |= ovLoseAll
	if !onBF(fl) {
		t.Fatal("a permanent that lost all abilities has no flanking")
	}
}
