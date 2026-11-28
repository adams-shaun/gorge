package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A copy effect newer than a pile's latest merge replaces the merge's rules
// text (CR 707.2, 613.2a, 730.2a), so the keywords with no KwMask bit and the
// ward of a card beneath the top go with it, as the mask keywords do
// (TestMutatedPileCopiedKeywords). The copied card's own are the pile's.

// copiedKwCards adds the copy templates to underKwTable's cards.
type copiedKwCards struct {
	underKwCards
	wurm, golem CardID // a trample-only creature; an artifact creature the ward granter reaches
	granter     CardID // underKwTable's ward {3} granter (grant tables only)
}

func copiedKwTable(grant bool) (*kop.Table, copiedKwCards) {
	tbl, base := underKwTable(grant)
	c := copiedKwCards{underKwCards: base}
	if grant {
		c.granter = CardID(slices.IndexFunc(tbl.Cards, func(cd kop.CardDef) bool { return cd.Name == "Test Ward Granter" }))
		tbl.Cards[c.granter].Abilities[0].Static.Kw = kop.KwWard // a ward grant sets the bit too (kop.Static.Ward)
	}
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	c.wurm = add(kop.CardDef{Name: "Copied Wurm", Types: kop.TCreature, Power: 6, Toughness: 6,
		Keywords: kop.KwTrample})
	c.golem = add(kop.CardDef{Name: "Copied Golem", Types: kop.TCreature | kop.TArtifact, Power: 3, Toughness: 3})
	addLaterCopyOp(tbl, kop.ExtCloneCopy)
	return tbl, c
}

// copyUntilEOT makes the permanent o a copy of card until end of turn, one
// copy layer newer than everything on it.
func copyUntilEOT(t *testing.T, sc *scen, o Obj, card CardID) {
	t.Helper()
	sl := sc.slot(o)
	addCopyEOT(sc, sl, card, NoObj, 0, sc.s.tick())
	setCopy(sc, sl, card)
	if got := sc.s.cardAt(sl); got != card {
		t.Fatalf("precondition: the pile shows card %d, want the copied %d", got, card)
	}
	if rec := sc.s.mergeRecOf(o); rec == nil || rec.n != 1 {
		t.Fatalf("precondition: the copy dropped the pile (%+v)", rec)
	}
}

// underKwReads is every non-mask keyword read of the permanent o.
type underKwReads struct {
	infect, wither, witherKw, skulk, skulkWord, infectWord bool
	withToxic, withHorse, withWard                         bool
	toxic                                                  int
	evasion                                                uint8
}

func readUnderKw(sc *scen, c copiedKwCards, o Obj) underKwReads {
	sl := sc.slot(o)
	with := func(id kop.KwID) bool {
		fc := fctx{k: sc.k, zone: kop.ZoneBattlefield, sl: sl}
		return sc.s.fwWithKeyword(&fc, &kop.FLit{Arg: int16(id)})
	}
	word := func(f kop.FilterID) bool { return sc.s.matchPermGen(sc.k, f, sl, 0, NoSlot, layerKw) }
	return underKwReads{
		infect: sc.s.infectAt(sc.k, sl), wither: sc.s.witherAt(sc.k, sl),
		witherKw: sc.s.witherKw(sc.k, sl, sc.s.kw(sc.k, sl)),
		skulk:    sc.s.hasSkulkPerm(sc.k, sl), skulkWord: word(c.skulkF), infectWord: word(c.infectF),
		withToxic: with(kop.KwToxic), withHorse: with(kop.KwHorsemanship), withWard: with(kop.KwPrintedWard),
		toxic: sc.s.toxicAt(sc.k, sl), evasion: sc.s.evasionAt(sc.k, sl),
	}
}

func TestCopiedPileDropsUnderNonMaskKeywords(t *testing.T) {
	tbl, c := copiedKwTable(false)
	for _, tc := range []struct {
		name  string
		under CardID
		want  underKwReads // the uncopied pile's reads
	}{
		{"infect and ward", c.plague, underKwReads{infect: true, infectWord: true, withWard: true}},
		{"wither and toxic", c.rot, underKwReads{wither: true, witherKw: true, withToxic: true, toxic: 2}},
		{"horsemanship", c.rider, underKwReads{withHorse: true, evasion: kop.EvHorsemanship}},
		{"intimidate", c.menace, underKwReads{evasion: kop.EvIntimidate}},
		{"skulk", c.sneak, underKwReads{skulk: true, skulkWord: true, evasion: kop.EvSkulk}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			b := underPile(t, sc, tc.under, c.bird)
			if got := readUnderKw(sc, c, b); got != tc.want {
				t.Fatalf("precondition: the pile reads %+v, want %+v", got, tc.want)
			}
			copyUntilEOT(t, sc, b, c.wurm)
			if got := readUnderKw(sc, c, b); got != (underKwReads{}) {
				t.Fatalf("copied pile reads %+v, want none of the under card's", got)
			}
			sc.s.cloneCleanup(sc.k)
			if got := readUnderKw(sc, c, b); got != tc.want {
				t.Fatalf("after the copy ended the pile reads %+v, want %+v", got, tc.want)
			}
		})
	}

	t.Run("the copied card's own", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		b := underPile(t, sc, c.rot, c.bird)
		copyUntilEOT(t, sc, b, c.plague)
		want := underKwReads{infect: true, infectWord: true, withWard: true}
		if got := readUnderKw(sc, c, b); got != want {
			t.Fatalf("copied pile reads %+v, want the copied card's %+v alone", got, want)
		}
	})

	t.Run("combat", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		b := underPile(t, sc, c.plague, c.bird)
		copyUntilEOT(t, sc, b, c.wurm)
		sc.combat(t, []Obj{b}, nil)
		if got, life := sc.s.Poison(1), sc.s.pl[1].life; got != 0 || life != 14 {
			t.Fatalf("defender poison %d life %d, want 0 and 14: the copied 6/6 has no infect", got, life)
		}
	})
}

func TestCopiedPileDropsUnderWard(t *testing.T) {
	const granted = int(3 | wardGrantedBits)
	for _, tc := range []struct {
		name         string
		grant        bool // the ward {3} granter is on the battlefield
		copied       func(c copiedKwCards) CardID
		want, wantAt []int // while copied; after the copy ends
	}{
		{"no ward left", false, func(c copiedKwCards) CardID { return c.wurm }, nil, []int{2}},
		{"the copied card's ward", false, func(c copiedKwCards) CardID { return c.warden }, []int{1}, []int{2}},
		{"the copied card's ward beside a ward grant table", true,
			func(c copiedKwCards) CardID { return c.warden }, []int{1}, []int{2}},
		{"a granted ward", true, func(c copiedKwCards) CardID { return c.golem }, []int{granted}, []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, c := copiedKwTable(tc.grant)
			for _, ended := range []bool{false, true} {
				sc := newScenTable(t, tbl, Config{NoElide: true})
				b := underPile(t, sc, c.plague, c.bird)
				if tc.grant {
					if !sc.k.gen.anyWardGrant {
						t.Fatalf("precondition: the table has no ward grant")
					}
					sc.perm(c.granter, 0)
				}
				copyUntilEOT(t, sc, b, tc.copied(c))
				want := tc.want
				if got := sc.s.kw(sc.k, sc.slot(b))&kop.KwWard != 0; !ended && got != (want != nil) {
					t.Fatalf("precondition: the copied pile has the ward bit %v, want %v", got, want != nil)
				}
				if ended {
					sc.s.cloneCleanup(sc.k)
					if got := sc.s.cardAt(sc.slot(b)); got != c.bird {
						t.Fatalf("precondition: the copy did not end: card %d", got)
					}
					want = tc.wantAt
				}
				if amts, _ := boltAsksWard(t, sc, c.bolt, b); !slices.Equal(amts, want) {
					t.Fatalf("copy ended %v: ward triggers %v, want %v", ended, amts, want)
				}
			}
		})
	}
}

// TestMergeUnderCopiedPermanentKeepsUnderNonMaskKeywords is the control for
// the order: a merge newer than a permanent copy lends the under card's
// keywords and ward, though the pile still shows the copy (CR 730.2a, 613.2a).
func TestMergeUnderCopiedPermanentKeepsUnderNonMaskKeywords(t *testing.T) {
	tbl, c := copiedKwTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	b := sc.perm(c.bird, 0)
	tok := sc.s.newToken(c.wurm, 0)
	sc.s.enter(sc.k, tok, 0, false, kop.ZoneNone)
	sc.s.npend = 0
	addCopyPerm(sc, sc.slot(b), tok, 0, sc.s.tick())
	setCopy(sc, sc.slot(b), c.wurm)
	mutateCast(t, sc, sc.hand(c.plague, 0), b, false)
	resolveStack(t, sc)
	if sl := sc.slot(b); sc.s.cardAt(sl) != c.wurm || sc.s.perms[sl].ovr.flags&ovCopy == 0 {
		t.Fatalf("precondition: want the plague under the wurm copy, shows card %d", sc.s.cardAt(sl))
	}
	if rec := sc.s.mergeRecOf(b); rec == nil || rec.n != 1 {
		t.Fatalf("precondition: the pile did not form (%+v)", rec)
	}
	want := underKwReads{infect: true, infectWord: true, withWard: true}
	if got := readUnderKw(sc, c, b); got != want {
		t.Fatalf("merged under the copy the pile reads %+v, want %+v", got, want)
	}
	if amts, _ := boltAsksWard(t, sc, c.bolt, b); !slices.Equal(amts, []int{2}) {
		t.Fatalf("ward triggers %v, want the under card's [2]", amts)
	}
}
