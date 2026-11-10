package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantBeCastOrigin: a StatCantBeCast whose ValidCard$ reads the cast
// origin (Drannith Magistrate / Avatar's Wrath: Card.!wasCastFromTheirHand,
// PredWasCastSpell PredN 17) is read at the cast offer from the candidate's
// current zone: the opponent's hand cast stays offered, the exile, graveyard
// and library ones are withheld, and the controller is never bound. The
// positive form (PredN 3, from exile) withholds only the exile cast.
func TestCantBeCastOrigin(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	filt := func(n int16) kop.FilterID {
		tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredWasCastSpell, PredN: n})
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	notHand, fromExile := filt(1|16), filt(3)
	lock, exileLock, spell := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Magistrate", Types: kop.TCreature, Power: 1, Toughness: 3, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeCast, StFlags: kop.StRaiseOpp, Filter: notHand}}}},
		kop.CardDef{Name: "Test Exile Lock", Types: kop.TCreature, Power: 1, Toughness: 3, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeCast, StFlags: kop.StRaiseOpp, Filter: fromExile}}}},
		kop.CardDef{Name: "Test Spell", Types: kop.TInstant})
	zones := []kop.Zone{kop.ZoneHand, kop.ZoneExile, kop.ZoneGraveyard, kop.ZoneLibrary}

	for _, c := range []struct {
		name string
		card CardID
		want func(z kop.Zone) bool // the opponent's cast is forbidden
	}{
		{"!wasCastFromTheirHand", lock, func(z kop.Zone) bool { return z != kop.ZoneHand }},
		{"wasCastFromExile", exileLock, func(z kop.Zone) bool { return z == kop.ZoneExile }},
	} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if !sc.k.gen.anyCantCast {
			t.Fatal("anyCantCast not set")
		}
		sc.perm(c.card, 0)
		for _, z := range zones {
			o := sc.obj(spell, 1)
			sc.s.objs[o].zone = z
			if got := sc.s.castForbidden(sc.k, 1, spell, o); got != c.want(z) {
				t.Errorf("%s: opponent's cast from zone %d forbidden=%v, want %v", c.name, z, got, c.want(z))
			}
			if sc.s.castForbidden(sc.k, 0, spell, o) {
				t.Errorf("%s: the controller's cast from zone %d is forbidden", c.name, z)
			}
		}
		// no object (unit-test call sites): fails closed, no panic
		_ = sc.s.castForbidden(sc.k, 1, spell, NoObj)

		// mutation: the provenance read ignored makes every clause hold, so
		// the hand cast is forbidden too (the exile static: all of them)
		DebugRestrProvIgnored = true
		o := sc.obj(spell, 1)
		sc.s.objs[o].zone = kop.ZoneHand
		hand := sc.s.castForbidden(sc.k, 1, spell, o)
		DebugRestrProvIgnored = false
		if !hand {
			t.Errorf("%s: DebugRestrProvIgnored did not change the hand cast", c.name)
		}
	}
}
