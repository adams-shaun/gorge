package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// unlessCensusBoard builds a manarider scenario with one permanent of each
// named card, plus a first-only filter ("{1}, {T}: Add one mana of any
// color") appended as a synthetic card. The fixture pool carries Gemstone
// Mine, the choice-shaped rider the unless census leaves out.
func unlessCensusBoard(t *testing.T, names ...string) (*scen, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "manarider")
	pool, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *pool
	tbl.Cards = append([]kop.CardDef(nil), pool.Cards...)
	filter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Filter", Types: kop.TArtifact, Abilities: []kop.Ability{{
		Kind:       kop.AbMana,
		Cost:       kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}},
		ManaColors: kop.ColAny,
	}}})
	id := map[string]CardID{}
	for c := range pool.Cards {
		if _, ok := id[pool.Cards[c].Name]; !ok {
			id[pool.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, &tbl, id["Plains"], Config{NoElide: true})
	for _, n := range names {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the manarider table", n)
		}
		sc.perm(c, 0)
	}
	return sc, id, filter
}

// TestFiltPaySkipsChoiceRider: the unless / ward census leaves a
// choice-shaped rider out (riderCensusFree fails closed), so the filter
// payment probe must not price Gemstone Mine's "{T}, Remove a mining counter:
// Add one mana of any color" either. The only way an unless {G} is payable is
// the rider funding a first-only filter's {1} (Gemstone Mine {G}, filter
// {G}); a direct probe (no unless flag) pins the gap set itself.
func TestFiltPaySkipsChoiceRider(t *testing.T) {
	sc, id, filter := unlessCensusBoard(t, "Gemstone Mine")
	sl := sc.slot(sc.perm(filter, 0))
	mine := sc.slot(sc.perm(id["Gemstone Mine"], 0))
	if !sc.k.gen.anyFiltFirst {
		t.Fatal("precondition: the table lists no first-only filter")
	}
	a, ok := sc.s.riderAbilityOf(sc.k, mine)
	if !ok {
		t.Fatal("precondition: Gemstone Mine is no bounded rider")
	}
	if riderCensusFree(a) {
		t.Fatal("precondition: Gemstone Mine's choice-shaped rider is census-free")
	}
	pm := &sc.s.perms[mine]
	pm.subCtrN(sc.s, int(a.ManaRiderCtr), pm.ctrN(sc.s, int(a.ManaRiderCtr)))
	if !pm.addCtrN(sc.s, int(a.ManaRiderCtr), 3) {
		t.Fatal("precondition: no mining counters on Gemstone Mine")
	}
	pm.flags &^= pfTapped
	if !sc.s.scriptFilt(sc.k, 0).has(sl) {
		t.Fatalf("precondition: the Test Filter is no live census gap (first %v usable %v fee %d otherMana %d filt %v)", sc.s.filtFirstOnly(sc.k, sl), sc.s.filtUsable(sc.k, sl), sc.s.filtFee(sc.k, sl), sc.s.filtOtherMana(sc.k, 0), sc.s.scriptFilt(sc.k, 0))
	}

	cost := manaOf(0, 1<<(3*kop.G))
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); ok {
		t.Fatal("the payment probe priced Gemstone Mine's choice-shaped rider")
	}

	// The census control: a Forest pays {G} itself, so the probe pays.
	sc.perm(id["Forest"], 0)
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); !ok {
		t.Fatal("the payment probe did not pay {G} beside a Forest")
	}
}

// unlessHandGuideBoard builds a spiritguide scenario (the pool's hand guides)
// with a synthetic first-only filter.
func unlessHandGuideBoard(t *testing.T) (*scen, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "spiritguide")
	pool, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *pool
	tbl.Cards = append([]kop.CardDef(nil), pool.Cards...)
	filter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Filter", Types: kop.TArtifact, Abilities: []kop.Ability{{
		Kind:       kop.AbMana,
		Cost:       kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}},
		ManaColors: kop.ColAny,
	}}})
	id := map[string]CardID{}
	for c := range pool.Cards {
		if _, ok := id[pool.Cards[c].Name]; !ok {
			id[pool.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, &tbl, id["Plains"], Config{NoElide: true})
	return sc, id, filter
}

// TestFiltPaySkipsHandGuide: the unless census prices no hand mana
// (unlessWindowUnits), so the filter payment probe must not reach handFund.
// Simian Spirit Guide's {R} is the only way the cost is paid; a Mountain is
// the census control.
func TestFiltPaySkipsHandGuide(t *testing.T) {
	sc, id, _ := unlessHandGuideBoard(t)
	if !sc.k.gen.anyFiltFirst {
		t.Fatal("precondition: the table lists no first-only filter")
	}
	if !handManaOn(sc.k) {
		t.Fatal("precondition: the table has no hand guide")
	}
	guide := sc.hand(id["Simian Spirit Guide"], 0)
	if sc.s.objs[guide].zone != kop.ZoneHand {
		t.Fatal("precondition: the guide is not in hand")
	}
	cost := manaOf(0, 1<<(3*kop.R))
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); ok {
		t.Fatal("the payment probe priced Simian Spirit Guide's hand mana")
	}

	// The census control: a Mountain makes {R}, so the probe pays.
	sc.perm(id["Mountain"], 0)
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); !ok {
		t.Fatal("the payment probe did not pay {R} beside a Mountain")
	}
}
