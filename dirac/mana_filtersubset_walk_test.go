package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// filterWalkScen builds a pain-fixture table with the given custom cards
// appended, puts the listed custom permanents and wastes Wastes on the
// battlefield, and puts the custom spell in hand. The fixture's Wastes is
// the library card and the generic source.
func filterWalkScen(t *testing.T, defs []kop.CardDef, spell kop.CardDef, permanents []int, wastes int) (*scen, []Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "pain")
	deck := []cardset.Entry{{Name: "Wastes", Count: 8}, {Name: "Adarkar Wastes", Count: 3}}
	tbl, da, _, err := set.Matchup(deck, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	base := len(tbl.Cards)
	tbl.Cards = append(slices.Clone(tbl.Cards), defs...)
	tbl.Cards = append(tbl.Cards, spell)
	sc := newScenTableLib(t, tbl, id["Wastes"], Config{NoElide: true})
	var objs []Obj
	for _, i := range permanents {
		objs = append(objs, sc.perm(CardID(base+i), 0))
	}
	for i := 0; i < wastes; i++ {
		sc.perm(id["Wastes"], 0)
	}
	return sc, objs, sc.hand(CardID(base+len(defs)), 0)
}

// filtLandDef is Henge of Ramos's shape: a free {T}: Add {C} and a paid
// {fee},{T}: Add one mana of any colour. It makes one mana for fee mana, so
// it is net-negative in the relaxed proof.
func filtLandDef(fee uint8) kop.CardDef {
	any := kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG
	return kop.CardDef{
		Types:  kop.TLand,
		Timing: kop.TimingLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Mana: kop.ManaCost{Generic: fee}, Flags: kop.CostTap}, ManaColors: any, ManaN: 1},
		},
		NoAbilities:    true,
		HasManaAbility: true,
		NoManaCost:     true,
		NoPT:           true,
		ProduceCol:     any,
	}
}

// painLandDef is a painland: a free {T}: Add {C} and a {T}: Add {W} that
// deals 1 damage to its controller. Three of them at 3 life sum to the
// payer's life, so the gaps walk's summed-lethal guard binds.
func painLandDef() kop.CardDef {
	return kop.CardDef{
		Types:  kop.TLand,
		Timing: kop.TimingLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1,
				Prog:   []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}},
				AFlags: kop.AfManaChoice},
		},
		NoAbilities:    true,
		HasManaAbility: true,
		NoManaCost:     true,
		NoPT:           true,
		ProduceCol:     kop.ColW,
	}
}

// TestGapsWalkThreeNetNegativeFilters: gorge's relaxed proof enumerates all
// 2^3 subsets of three paid filters. Two {2},{T} filters pay {2}{R}{R}; the
// third is a {10},{T} filter whose fee makes every run that keeps it short.
// The kernel's gaps walk stopped after the first two net-negative filters,
// so every run kept the expensive third one and the cast was dropped.
func TestGapsWalkThreeNetNegativeFilters(t *testing.T) {
	spellDef := kop.CardDef{Cost: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{0, 0, 0, 2}}, Types: kop.TSorcery, Colors: kop.ColR, NoPT: true}
	sc, filters, spell := filterWalkScen(t, []kop.CardDef{filtLandDef(2), filtLandDef(2), filtLandDef(10)}, spellDef, []int{0, 1, 2}, 6)
	if got := sc.s.scriptFilt(sc.k, 0).count(); got != 3 {
		t.Fatalf("%d filter gaps, want 3", got)
	}
	for _, f := range filters {
		sl := sc.slot(f)
		if u, fee := sc.s.filtUnits(sc.k, sl), sc.s.filtFee(sc.k, sl); u >= fee {
			t.Fatalf("filter %v makes %d for %d: not net-negative", f, u, fee)
		}
	}
	// The custom spell is the last def appended: {2}{R}{R}. The run that
	// leaves out the third filter and keeps the first two pays it, so the
	// gaps walk lists it as a candidate.
	a := MakeAction(AkCast, TCode(spell), 0, 0)
	var gaps Decision
	sc.s.plainGapsOptions(sc.k, &gaps, sc.s.scriptFilt(sc.k, 0), 0)
	if !hasOption(&gaps, a) {
		t.Fatal("the gaps walk never kept both {2},{T} filters: the cast is not a candidate")
	}
}

// TestGapsWalkPainGuard: the gaps walk keeps the summed-lethal life guard.
// Two filters make the all-in run one mana short, and the run that keeps one
// filter needs all three painlands' white, which at 3 life would kill the
// payer. The walk must not list that cast as a candidate.
func TestGapsWalkPainGuard(t *testing.T) {
	scen := func(life int16) (*scen, Obj) {
		spellDef := kop.CardDef{Cost: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{4}}, Types: kop.TSorcery, Colors: kop.ColW, NoPT: true}
		sc, _, spell := filterWalkScen(t, []kop.CardDef{filtLandDef(2), filtLandDef(2), painLandDef(), painLandDef(), painLandDef()}, spellDef, []int{0, 1, 2, 3, 4}, 2)
		sc.s.pl[0].life = life
		if got := sc.s.scriptFilt(sc.k, 0).count(); got != 2 {
			t.Fatalf("%d filter gaps, want 2", got)
		}
		return sc, spell
	}
	// At 20 life the one-filter run pays the cast, so the walk lists it.
	control, spell := scen(20)
	a := MakeAction(AkCast, TCode(spell), 0, 0)
	var gaps Decision
	control.s.plainGapsOptions(control.k, &gaps, control.s.scriptFilt(control.k, 0), 0)
	if !hasOption(&gaps, a) {
		t.Fatal("control board at 20 life does not list the {2}{W}{W}{W}{W} cast")
	}
	// At 3 life all three painlands together are lethal; the one-filter run
	// needs all three, so the gaps walk must not list the cast at all.
	sc, spell3 := scen(3)
	var tight Decision
	sc.s.plainGapsOptions(sc.k, &tight, sc.s.scriptFilt(sc.k, 0), 0)
	if hasOption(&tight, MakeAction(AkCast, TCode(spell3), 0, 0)) {
		t.Fatal("the gaps walk lists the cast by counting three painlands at 3 life")
	}
}
