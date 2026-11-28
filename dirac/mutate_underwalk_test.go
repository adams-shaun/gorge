package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A merged permanent has the landwalk and the protection of every card in its
// pile (CR 702.140e), not only of its top card. Each pile here is an under
// card mutated beneath the vanilla bird, which prints neither; a plain bird
// beside it is the control.

// underWalkCards are underWalkTable's cards.
type underWalkCards struct {
	bird, bolt          CardID
	walker, ward, scarl CardID // under cards: landwalk, protection from instants, protection from red
	landF, instF        kop.FilterID
}

func underWalkTable() (*kop.Table, underWalkCards) {
	tbl, c0 := underKwTable(false)
	c := underWalkCards{bird: c0.bird, bolt: c0.bolt}
	filter := func(f kop.Filter) kop.FilterID {
		id := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, f)
		return id
	}
	c.landF = filter(kop.Filter{Types: kop.TLand})
	c.instF = filter(kop.Filter{Types: kop.TInstant})
	mut := tbl.Cards[c0.plague] // a mutate creature: its mutate fields
	under := func(name string, set func(cd *kop.CardDef)) CardID {
		cd := kop.CardDef{Name: name, Types: kop.TCreature, Power: 1, Toughness: 1,
			HasMutate: true, MutateCost: mut.MutateCost, MutateTgt: mut.MutateTgt}
		set(&cd)
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, cd)
		return id
	}
	c.walker = under("Test Walker", func(cd *kop.CardDef) { cd.Landwalk = c.landF })
	c.ward = under("Test Spellward", func(cd *kop.CardDef) { cd.ProtFrom = c.instF })
	c.scarl = under("Test Scarlet", func(cd *kop.CardDef) { cd.ProtColors = kop.ColR })
	return tbl, c
}

func TestMutateUnderCardLandwalkAndProtection(t *testing.T) {
	tbl, c := underWalkTable()
	if cd := tbl.Cards[c.bird]; cd.Landwalk != 0 || cd.ProtFrom != 0 || cd.ProtColors != 0 {
		t.Fatalf("precondition: the top bird prints landwalk %d protection %d/%b", cd.Landwalk, cd.ProtFrom, cd.ProtColors)
	}
	pile := func(t *testing.T, under CardID) (sc *scen, b, plain Obj) {
		t.Helper()
		sc = newScenTable(t, tbl, Config{NoElide: true})
		b = underPile(t, sc, under, c.bird)
		return sc, b, sc.perm(c.bird, 0)
	}

	t.Run("landwalk", func(t *testing.T) {
		sc, b, plain := pile(t, c.walker)
		blk := sc.perm(c.bird, 1)
		sc.perm(v.Island, 1) // the defending player's land the walk names
		if sc.s.landwalkEvades(sc.k, sc.slot(blk), sc.slot(plain)) {
			t.Fatalf("precondition: the plain bird evades the blocker")
		}
		if !sc.s.landwalkEvades(sc.k, sc.slot(blk), sc.slot(b)) {
			t.Fatalf("the blocker can block the pile despite its under card's landwalk")
		}
		if sc.s.hasLandwalkSpec(sc.k, sc.slot(plain), c.landF) || !sc.s.hasLandwalkSpec(sc.k, sc.slot(b), c.landF) {
			t.Fatalf("hasLandwalkSpec: plain %v pile %v, want false and true",
				sc.s.hasLandwalkSpec(sc.k, sc.slot(plain), c.landF), sc.s.hasLandwalkSpec(sc.k, sc.slot(b), c.landF))
		}
	})

	t.Run("protection quality", func(t *testing.T) {
		sc, b, plain := pile(t, c.ward)
		q := func(o Obj) bool { return sc.s.protQuality(sc.k, sc.slot(o), NoSlot, c.bolt) }
		if q(plain) || !q(b) {
			t.Fatalf("protection from instants: plain %v pile %v, want false and true", q(plain), q(b))
		}
		h := func(o Obj) bool { return sc.s.hasProtQuality(sc.k, sc.slot(o), c.instF) }
		if h(plain) || !h(b) {
			t.Fatalf("hasProtQuality: plain %v pile %v, want false and true", h(plain), h(b))
		}
		for i := 0; i < 2; i++ {
			sc.perm(v.Mountain, 1)
		}
		o := sc.hand(c.bolt, 1)
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkCast, TCode(o), scAny)
		d := sc.s.Step()
		offered := func(x Obj) bool {
			for _, a := range d.Options[:d.N] {
				if a.Kind() == AkTarget && a.Target() == TCode(x) {
					return true
				}
			}
			return false
		}
		if !offered(plain) || offered(b) {
			t.Fatalf("bolt targets: plain %v pile %v, want true and false", offered(plain), offered(b))
		}
	})

	t.Run("protection colour", func(t *testing.T) {
		sc, b, plain := pile(t, c.scarl)
		p := func(o Obj) bool { return sc.s.protColFrom(sc.k, sc.slot(o), kop.ColR) }
		if p(plain) || !p(b) {
			t.Fatalf("protection from red: plain %v pile %v, want false and true", p(plain), p(b))
		}
		if !sc.s.hasProtColor(sc.k, sc.slot(b), kop.ColR) || sc.s.hasProtColor(sc.k, sc.slot(b), kop.ColU) {
			t.Fatalf("hasProtColor: the pile's red %v blue %v, want true and false",
				sc.s.hasProtColor(sc.k, sc.slot(b), kop.ColR), sc.s.hasProtColor(sc.k, sc.slot(b), kop.ColU))
		}
		// the oracle drops only the top face's printed spellings, so the
		// under card's protection from red projects as the keyword bit
		var pr Projection
		sc.s.Project(&pr)
		sc.s.ProjectChars(&pr)
		if pr.Chars[plain].Kw&kop.KwProtRed != 0 || pr.Chars[b].Kw&kop.KwProtRed == 0 {
			t.Fatalf("projected KwProtRed: plain %v pile %v, want false and true",
				pr.Chars[plain].Kw&kop.KwProtRed != 0, pr.Chars[b].Kw&kop.KwProtRed != 0)
		}
	})
}
