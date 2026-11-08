package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fixedTapsScen is a scenario over a table holding deck's cards.
func fixedTapsScen(t *testing.T, pool string, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, pool)
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

// untappedPerm puts c onto p's battlefield untapped (an enters-tapped land too).
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
	sc, id := fixedTapsScen(t, "std-dsk", []cardset.Entry{
		{Name: "Fear of Exposure", Count: 1}, {Name: "Patchwork Beastie", Count: 1}, {Name: "Forest", Count: 20},
	})
	k := sc.k
	fe := sc.hand(id["Fear of Exposure"], 0)
	if !spellFixedTapsOnly(k, id["Fear of Exposure"]) || spellFixedTapsOnly(k, id["Patchwork Beastie"]) {
		t.Fatal("spellFixedTapsOnly: Fear of Exposure only")
	}
	a := MakeAction(AkCast, uint8(fe), 0, 0)
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
	sc.untappedPerm(id["Patchwork Beastie"], 0)
	d := sc.s.grantPayDesc(a)
	if sc.s.affordsUngrantedFor(k, 0, &cost, NoSlot, d) == false {
		t.Fatal("precondition: three Forests pay {2}{G}")
	}
	if sc.s.affordsUngrantedPlay(k, 0, a, &cost, NoSlot, d) {
		t.Error("three Forests and one creature: the tap-two part priced with a Forest the mana spends")
	}
	sc.untappedPerm(id["Patchwork Beastie"], 0)
	if !sc.s.affordsUngrantedPlay(k, 0, a, &cost, NoSlot, d) {
		t.Error("three Forests and two creatures: unpriced")
	}
}

// TestFixedTapsRestrictedSourceIsNoFreeCandidate: Creeping Peeper's
// enchantment-only {U} pays Fear of Exposure (an enchantment creature spell),
// so tapping it for the tap-two part takes that mana away: a Forest, an Island,
// the Peeper and one other creature do not cast it (CR 601.2g-h).
func TestFixedTapsRestrictedSourceIsNoFreeCandidate(t *testing.T) {
	sc, id := fixedTapsScen(t, "std-dsk", []cardset.Entry{
		{Name: "Fear of Exposure", Count: 1}, {Name: "Creeping Peeper", Count: 1}, {Name: "Patchwork Beastie", Count: 1},
		{Name: "Island", Count: 1}, {Name: "Forest", Count: 20},
	})
	k := sc.k
	fe := sc.hand(id["Fear of Exposure"], 0)
	sc.untappedPerm(id["Forest"], 0)
	sc.untappedPerm(id["Island"], 0)
	peeper := sc.untappedPerm(id["Creeping Peeper"], 0)
	sc.untappedPerm(id["Patchwork Beastie"], 0)
	c := id["Fear of Exposure"]
	cost := sc.s.featCost(k, 0, c, fe, k.cards[c].cost)
	ac := spellAddCost(k, c)
	d := payDesc{class: pdSpell, card: c, src: NoSlot, og: sc.s.restrOriginOfObj(fe)}
	if _, ok := sc.s.restrTapUnit(k, 0, sc.slot(peeper), &d); !ok {
		t.Fatal("precondition: the Peeper's restricted {U} may pay Fear of Exposure")
	}
	if sc.s.spellPartDisjoint(k, 0, c, fe, ac, &cost) {
		t.Error("Forest, Island, Peeper and a creature: Fear of Exposure's mana and tap-two priced together")
	}
	sc.untappedPerm(id["Forest"], 0)
	if !sc.s.spellPartDisjoint(k, 0, c, fe, ac, &cost) {
		t.Error("two Forests, Island, Peeper and a creature: unpriced")
	}
}

// TestFixedTapsPlainPlanFirst: nine lands cast Guardian of the Great Door: the
// plain plan pays {W}{W} from the Plains and Restless Anchorage and leaves seven
// untapped (gorge planReserving's first step), however many reservations the
// enumeration would try before it reached one keeping both {W} sources
// (CR 601.2g-h).
func TestFixedTapsPlainPlanFirst(t *testing.T) {
	sc, id := fixedTapsScen(t, "std-lci", []cardset.Entry{
		{Name: "Guardian of the Great Door", Count: 1}, {Name: "Plains", Count: 1}, {Name: "Restless Anchorage", Count: 1},
		{Name: "Mountain", Count: 1}, {Name: "Forest", Count: 20},
	})
	k := sc.k
	gd := sc.hand(id["Guardian of the Great Door"], 0)
	// the {W} sources first, so the slot-order enumeration reserves them first
	sc.untappedPerm(id["Plains"], 0)
	sc.untappedPerm(id["Restless Anchorage"], 0)
	for i := 0; i < 7; i++ {
		sc.untappedPerm(id["Mountain"], 0)
	}
	c := id["Guardian of the Great Door"]
	cost := sc.s.featCost(k, 0, c, gd, k.cards[c].cost)
	ac := spellAddCost(k, c)
	var av avail
	sc.s.availFor(k, 0, &av)
	if sc.s.costPartDisjoint(k, 0, NoSlot, ac, &cost, &av, pipMaskOf(&cost)) {
		t.Fatal("precondition: the reservation enumeration finds an assignment within its limit")
	}
	if !sc.s.spellPartDisjoint(k, 0, c, gd, ac, &cost) {
		t.Error("nine lands: Guardian of the Great Door unpriced")
	}
}
