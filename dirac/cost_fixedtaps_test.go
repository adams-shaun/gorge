package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fixedTapsScen is a scenario over the planshape pool's table (Fear of
// Exposure, Grizzly Bears, Forest, Island). The restricted-source and plain-plan
// rules (cost_fixedtaps.go) need cards no fixture pool holds: their lockstep
// fixtures tapxtype_restricted_source_not_free (std-dsk) and
// tapxtype_plain_plan_leaves_candidates (std-lci) pin them.
func fixedTapsScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	const pool = "planshape"
	set := cardsettest.Set(t, pool)
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, e := range deck {
		if _, ok := id[e.Name]; !ok {
			t.Fatalf("%s not in the %s table", e.Name, pool)
		}
	}
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true}), id
}

// untappedPerm puts c onto p's battlefield untapped.
func (sc *scen) untappedPerm(c CardID, p uint8) Obj {
	o := sc.perm(c, p)
	sc.s.perms[sc.slot(o)].flags &^= pfTapped
	return o
}

// TestFixedTapsSpellPricedFromComposedCost: a spell whose only extra is a fixed
// tap of other permanents (Fear of Exposure's tap-two) is priced from its composed
// cost by gorge's potential planner (spellExtrasAreFixedTaps), so the granted-mana
// search prices it at its printed {2}{G} -- and its terminal check keeps two
// candidates apart from the mana (CR 601.2g-h).
func TestFixedTapsSpellPricedFromComposedCost(t *testing.T) {
	sc, id := fixedTapsScen(t, []cardset.Entry{
		{Name: "Fear of Exposure", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Forest", Count: 20},
	})
	k := sc.k
	fe := sc.hand(id["Fear of Exposure"], 0)
	if !spellFixedTapsOnly(k, id["Fear of Exposure"]) || spellFixedTapsOnly(k, id["Grizzly Bears"]) {
		t.Fatal("spellFixedTapsOnly: Fear of Exposure only")
	}
	a := MakeAction(AkCast, TCode(fe), 0, 0)
	cost, _, _, ok := sc.s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("the granted-mana search declines Fear of Exposure's cast")
	}
	if cost.MV() != 3 || cost.Pips[kop.G] != 1 {
		t.Fatalf("priced at %+v, want {2}{G}", cost)
	}
	// three Forests pay the mana and two creatures are the taps; with one creature the
	// part's second candidate must be a Forest the mana needs
	for i := 0; i < 3; i++ {
		sc.untappedPerm(id["Forest"], 0)
	}
	sc.untappedPerm(id["Grizzly Bears"], 0)
	d := sc.s.grantPayDesc(k, a)
	if sc.s.affordsUngrantedFor(k, 0, &cost, NoSlot, d) == false {
		t.Fatal("precondition: three Forests pay {2}{G}")
	}
	if sc.s.affordsUngrantedPlay(k, 0, a, &cost, NoSlot, d) {
		t.Error("three Forests and one creature: the tap-two part priced with a Forest the mana spends")
	}
	sc.untappedPerm(id["Grizzly Bears"], 0)
	if !sc.s.affordsUngrantedPlay(k, 0, a, &cost, NoSlot, d) {
		t.Error("three Forests and two creatures: unpriced")
	}
}
