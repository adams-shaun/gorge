package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The printed-keyword filter words read a merged permanent's whole pile (CR
// 702.140e): gorge answers them over its derived keyword list, which holds the
// keyword lines of the cards beneath the top (chars.PrintedKeywords). Each
// fixture card mutates UNDER a bird, which prints none of the keywords.
func TestMutateUnderCardFilterWords(t *testing.T) {
	tbl, c := underKwTable(false)
	under := func(cd kop.CardDef) CardID {
		base := &tbl.Cards[c.plague]
		cd.Types, cd.Power, cd.Toughness = kop.TCreature, 1, 1
		cd.HasMutate, cd.MutateCost, cd.MutateTgt = true, base.MutateCost, base.MutateTgt
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, cd)
		return id
	}
	flanker := under(kop.CardDef{Name: "Test Flanker", Flanking: true})
	void := under(kop.CardDef{Name: "Test Void", Devoid: true})
	warper := under(kop.CardDef{Name: "Test Warper", HasWarp: true})
	disguised := under(kop.CardDef{Name: "Test Disguised", MorphFamily: kop.MorphDisguise})
	echo := under(kop.CardDef{Name: "Test Echo", Keywords: kop.KwFlashback})

	for _, tc := range []struct {
		name  string
		under CardID
		word  int16
		// pile is the word's answer on a pile with under beneath the bird;
		// a plain bird and a pile that lost its abilities answer !pile.
		pile bool
	}{
		{"withoutFlanking", flanker, kop.W7WithoutFlanking, false},
		{"withDevoid", void, kop.W7WithDevoid, true},
		{"withWarp", warper, kop.W7WithWarp, true},
		{"withDisguise", disguised, kop.W7WithDisguise, true},
		{"faceUp+withDisguise", disguised, kop.W7FaceUpDisguise, true},
		{"withFlashback", echo, kop.W7WithFlashback, true},
	} {
		f := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Pred: kop.PredWord7, PredN: tc.word})
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			b := underPile(t, sc, tc.under, c.bird)
			plain := sc.perm(c.bird, 0)
			has := func(o Obj) bool { return sc.s.matchPermGen(sc.k, f, sc.slot(o), 0, NoSlot, layerKw) }
			if got := has(plain); got == tc.pile {
				t.Fatalf("plain bird %v, want %v", got, !tc.pile)
			}
			if got := has(b); got != tc.pile {
				t.Fatalf("pile %v, want %v: the under card's keyword", got, tc.pile)
			}
			sc.s.perms[sc.slot(b)].ovr.flags |= ovLoseAll
			if got := has(b); got == tc.pile {
				t.Fatalf("a pile that lost its abilities answers %v, want %v", got, !tc.pile)
			}
		})
	}
}
