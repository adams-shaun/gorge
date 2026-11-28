package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMadnessCensusSkipsZeroYieldAndLockedRestrictedSources: the madness
// unpayability proof reads gorge's planner census (paymentPlanCensusOf), which
// walks only abilities that make something and may be activated. An admitted
// restricted ability whose dynamic amount reads zero is no gap there, and nor
// is one a CantBeActivated lock (Petrify) holds, so the census stays complete
// (CR 106.1, 602.5). One that yields mana the census cannot price still is.
func TestMadnessCensusSkipsZeroYieldAndLockedRestrictedSources(t *testing.T) {
	set := cardsettest.Set(t, "cantact")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		flags kop.AbilityFlags
		// noCounter makes the ability no restricted planner unit (Cavern of
		// Souls' AddsNoCounter$), so it is unpriced though it yields mana.
		noCounter bool
		locked    bool
		// complete is the census verdict before any lock.
		complete bool
	}{
		{name: "zero yield", flags: kop.AfManaDynamic, complete: true},
		{name: "unpriced yield locked", noCounter: true, locked: true, complete: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *tbl0
			tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
			tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
			yours := kop.FilterID(len(tbl.Filters) - 1)
			id := func(name string) CardID {
				i, ok := set.Lookup(name)
				if !ok {
					t.Fatalf("%s not in the cantact pool", name)
				}
				return CardID(set.PoolID(i))
			}
			spring := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Spring", Types: kop.TArtifact, Abilities: []kop.Ability{
				{Kind: kop.AbMana, AFlags: tc.flags, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU,
					ManaRestrict: yours, ManaRestrictAb: yours, ManaNoCounter: tc.noCounter},
			}})
			bear := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature})
			sc := newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true})
			s := sc.s

			o := sc.perm(spring, 0)
			sl := sc.slot(o)
			spell := sc.perm(bear, 0)

			a := &sc.k.t.Cards[spring].Abilities[0]
			d := payDesc{class: pdSpell, card: s.objs[spell].card, src: NoSlot, og: s.restrOriginOfObj(spell)}
			if sc.k.gen == nil || !s.restrSrcAdmits(sc.k, 0, sl, a, &d) || s.windowPricesRestr(sc.k, sl, a) {
				t.Fatalf("precondition: want an admitted, unpriced restricted ability (payment %+v)", d)
			}
			if got := s.dynManaYieldsNothing(sc.k, a, sl); got != tc.complete {
				t.Fatalf("precondition: yields nothing = %v, want %v", got, tc.complete)
			}
			if complete, _, _ := s.madnessDeferProofClass(sc.k, 0, spell); complete != tc.complete {
				t.Errorf("unlocked: census complete = %v, want %v", complete, tc.complete)
			}
			if !tc.locked {
				return
			}
			sc.wear(id("Petrify"), o)
			if !s.actForbidden(sc.k, sl, true) {
				t.Fatal("precondition: no CantBeActivated lock")
			}
			if complete, relaxable, gains := s.madnessDeferProofClass(sc.k, 0, spell); !complete || !relaxable || gains {
				t.Errorf("locked: census (complete, relaxable, gains) = (%v, %v, %v), want (true, true, false)",
					complete, relaxable, gains)
			}
		})
	}
}
