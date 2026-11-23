package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dynFiltIntrinsicScen is a Cabal Coffers ("{2}, {T}: Add {B} for each Swamp
// you control") and three Plains under a grant making every permanent a land of
// the basic type sub, with a spell of {3}{W} in hand (Hill Giant, its cost reset to Indebted
// Samurai's).
func dynFiltIntrinsicScen(t *testing.T, sub string) (*scen, Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "lifefeemana")
	deck := []cardset.Entry{{Name: "Cabal Coffers", Count: 1}, {Name: "Plains", Count: 3}, {Name: "Hill Giant", Count: 1}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	idx := -1
	for j, st := range tbl.Subtypes {
		if st == sub {
			idx = j
		}
	}
	if idx < 0 {
		idx = len(tbl.Subtypes)
		tbl.Subtypes = append(tbl.Subtypes, sub)
	}
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == "Hill Giant" {
			tbl.Cards[c].Cost = kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{1}}
		}
	}
	var sm kop.SubMask
	sm[idx/64] |= 1 << (uint(idx) % 64)
	tbl.Overrides = append(tbl.Overrides, kop.Override{AddTypes: kop.TLand, AddSub: sm})
	tbl.Filters = append(tbl.Filters, kop.Filter{}) // matches every permanent
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Land Grant", Types: kop.TEnchantment, Abilities: []kop.Ability{{
		Kind: kop.AbStatic,
		Static: kop.Static{
			Kind:     kop.StatOverride,
			Filter:   kop.FilterID(len(tbl.Filters) - 1),
			Override: uint16(len(tbl.Overrides)),
		},
	}}})
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	coffers := sc.perm(id["Cabal Coffers"], 0)
	for i := 0; i < 3; i++ {
		sc.s.perms[sc.slot(sc.perm(id["Plains"], 0))].flags &^= pfTapped | pfSick
	}
	g := sc.perm(grant, 0)
	sc.s.perms[sc.slot(g)].flags |= pfTapped // the grant is itself a land of sub: tapped, it is no source
	sc.s.perms[sc.slot(coffers)].flags &^= pfTapped | pfSick
	if !sc.k.gen.blgOn || !sc.k.gen.anyDynFilt {
		t.Fatal("precondition: the table has no land-type grant or no dynamic paid filter")
	}
	sam := sc.hand(id["Hill Giant"], 0)
	if c := tbl.Cards[sc.s.cardAt(sc.slot(coffers))].Cost; c.Generic != 0 {
		t.Fatalf("precondition: Cabal Coffers is a land with a printed cost %+v", c)
	}
	return sc, coffers, sam
}

// TestDynFiltIntrinsicBoundary pins both sides of dynFiltIntrinsic against
// gorge's planner (engine/rules TestPayCoffersWithSwampGrantIsScriptedNotPlanned
// and its Forest control). A Cabal Coffers made a Swamp yields mana (its Swamp
// count is at least one), so gorge's census holds the filter and its intrinsic
// {B} out and the plain cast of {3}{W} from three Plains is no plain plan. Made
// a Forest, the filter's amount is zero: manaAbilityYieldsNothing drops it from
// the census, the Coffers' intrinsic {G} stays a planner unit, and the plain
// plan exists (CR 305.6, 605.3a).
func TestDynFiltIntrinsicBoundary(t *testing.T) {
	for _, tc := range []struct {
		sub          string
		wantIntr     bool
		wantPlanned  bool
		wantYieldsNo bool
	}{
		{"Swamp", true, false, false},
		{"Forest", false, true, true},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			sc, coffers, sam := dynFiltIntrinsicScen(t, tc.sub)
			sl := sc.slot(coffers)
			a := &sc.k.t.Cards[sc.s.cardAt(sl)].Abilities[sc.k.gen.filt(sc.s.cardAt(sl))]
			if got := sc.s.dynManaYieldsNothing(sc.k, a, sl); got != tc.wantYieldsNo {
				t.Fatalf("precondition: the Coffers' dynamic amount yields nothing = %v, want %v", got, tc.wantYieldsNo)
			}
			if got := sc.s.blgMask(sc.k, sl) != 0; !got {
				t.Fatal("precondition: the grant gives the Coffers no intrinsic mana ability")
			}
			if got := sc.s.dynFiltIntrinsic(sc.k, 0).has(sl); got != tc.wantIntr {
				t.Fatalf("dynFiltIntrinsic holds the Coffers = %v, want %v", got, tc.wantIntr)
			}
			if got := sc.s.FeatPlannedCast(0, sam, 0); got != tc.wantPlanned {
				t.Fatalf("the {3}{W} cast is a planned cast = %v, want %v", got, tc.wantPlanned)
			}
		})
	}
}
