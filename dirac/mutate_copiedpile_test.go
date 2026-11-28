package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutatedPileCopiedKeywords is the kernel mirror of gorge's
// Object.CopiedSinceMerge (CR 707.2, 613.2a, 730.2a): the merge and a copy are
// layer-1 copiable effects in timestamp order, so a copy newer than the merge
// takes the under cards' keywords off the pile, and a merge newer than the
// copy lends them beside the copied card's.
func TestMutatedPileCopiedKeywords(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	tbl.Cards[mutator].Keywords = kop.KwReach
	tbl.Cards[bird].Keywords = kop.KwFlying
	card := func(name string, kw kop.KwMask) CardID {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TCreature,
			Power: 6, Toughness: 6, Keywords: kw})
		return CardID(len(tbl.Cards) - 1)
	}
	wurm := card("Copied Wurm", kop.KwTrample)
	mole := card("Copied Mole", kop.KwDeathtouch)
	addLaterCopyOp(tbl, kop.ExtCloneCopy)

	// setup returns a game with the bird on the battlefield and the two copy
	// templates as tokens.
	setup := func(t *testing.T) (sc *scen, host, wurmTok, moleTok Obj) {
		t.Helper()
		sc = newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		host = sc.perm(bird, 0)
		tok := func(c CardID) Obj {
			o := sc.s.newToken(c, 0)
			sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
			sc.s.npend = 0
			return o
		}
		return sc, host, tok(wurm), tok(mole)
	}
	merge := func(t *testing.T, sc *scen, host Obj, top bool) {
		t.Helper()
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, host, top)
		resolveStack(t, sc)
		if rec := sc.s.mergeRecOf(host); rec == nil || rec.n != 1 {
			t.Fatalf("precondition: the pile did not form (%+v)", rec)
		}
	}
	// copyEOTOnto and copyPermOnto apply one copy layer now.
	copyEOTOnto := func(sc *scen, host Obj, c CardID) {
		sl := sc.slot(host)
		addCopyEOT(sc, sl, c, NoObj, 0, sc.s.tick())
		setCopy(sc, sl, c)
	}
	copyPermOnto := func(sc *scen, host, src Obj) {
		sl := sc.slot(host)
		addCopyPerm(sc, sl, src, 0, sc.s.tick())
		setCopy(sc, sl, sc.s.objs[src].card)
	}
	want := func(t *testing.T, sc *scen, host Obj, c CardID, kw kop.KwMask, when string) {
		t.Helper()
		sl := sc.slot(host)
		if got := sc.s.cardAt(sl); got != c {
			t.Fatalf("precondition (%s): the pile shows card %d, want %d", when, got, c)
		}
		if got := sc.s.kwGen(sc.k, sl); got != kw {
			t.Fatalf("%s: keywords %#x, want %#x", when, got, kw)
		}
	}

	t.Run("a copy newer than the merge", func(t *testing.T) {
		sc, host, _, _ := setup(t)
		merge(t, sc, host, true)
		want(t, sc, host, mutator, kop.KwReach|kop.KwFlying, "merged")
		copyEOTOnto(sc, host, wurm)
		want(t, sc, host, wurm, kop.KwTrample, "copied")
		sc.s.cloneCleanup(sc.k)
		want(t, sc, host, mutator, kop.KwReach|kop.KwFlying, "copy ended")
	})

	t.Run("a merge newer than the copy", func(t *testing.T) {
		sc, host, wurmTok, _ := setup(t)
		copyPermOnto(sc, host, wurmTok)
		merge(t, sc, host, false)
		want(t, sc, host, wurm, kop.KwTrample|kop.KwReach, "merged under the copy")
		copyEOTOnto(sc, host, mole)
		want(t, sc, host, mole, kop.KwDeathtouch, "copied again")
		sc.s.cloneCleanup(sc.k)
		want(t, sc, host, wurm, kop.KwTrample|kop.KwReach, "back on the older copy")
	})

	t.Run("a permanent copy newer than the merge outlasts a later one", func(t *testing.T) {
		sc, host, _, moleTok := setup(t)
		merge(t, sc, host, true)
		copyPermOnto(sc, host, moleTok)
		copyEOTOnto(sc, host, wurm)
		want(t, sc, host, wurm, kop.KwTrample, "copied twice")
		sc.s.cloneCleanup(sc.k)
		want(t, sc, host, mole, kop.KwDeathtouch, "back on the newer permanent copy")
	})
}
