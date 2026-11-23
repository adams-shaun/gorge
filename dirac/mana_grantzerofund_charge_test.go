package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantZeroFiltTapChargesOnce: grantSrch's funding loop taps Three Tree
// City's plain {T}: {C} beside its zero-amount dynamic any-colour filter with
// no ask of its own (the census ability is the one-bit {C}), so the tap must
// charge the zero-filter ask once itself, like the filter search does before
// its one answer (mana_filtzerofund.go): otherwise dirac spends one node fewer
// per tap than gorge and can reach a script at the budget edge that gorge
// never reaches. The count is one node lower per gap-side City activation
// than before the flattened-ask charge fix (152): a City activation whose
// "choose a mana ability" ask was charged asks its filter's colours inside
// that one ask, as gorge's scriptOrderManaAbilityAnswers flattens them
// (mana_grantedoffers.go filtBranch's flatAsk), so the colour ask is no node
// of its own there; the funding tap's zero-filter ask below is untouched.
func TestGrantZeroFiltTapChargesOnce(t *testing.T) {
	set := cardsettest.Set(t, "dynfilter")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	grantor := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grantor", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: kop.FilterID(len(tbl.Filters) - 1), Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1}}})
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	s, k := sc.s, sc.k
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	ttc := sc.perm(id["Three Tree City"], 0)
	sc.perm(id["Starfleet Academy"], 0)
	sc.perm(id["Grizzly Bears"], 0)
	sc.perm(grantor, 0)
	s.pl[0].pool[kop.C] += 2
	var gs grantSrch
	s.grantSearch(k, 0, &kop.ManaCost{Generic: 8}, NoSlot, payDesc{}, [kop.NumManaKinds]bool{}, &gs)
	tsl := sc.slot(ttc)
	if !k.gen.anyDynFilt || !k.gen.has(id["Three Tree City"], gfFilt) || s.filtUnits(k, tsl) != 0 || gs.nodes != 147 {
		t.Fatalf("want the zero-filter shape and 147 nodes (152 before the flattened-ask charge), got %d", gs.nodes)
	}
}
