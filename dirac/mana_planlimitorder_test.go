package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlanLimitChoiceCensusOrder is the node-limited plan's alternative order
// for a printed choice-shaped mana land on a board with a granted basic land
// type (mana_planlimitorder.go unitAltsChoice): gorge's census keeps only
// deterministic production in its shared walk (rules/mana_available.go
// windowManaUnitsWith skips every any-counted ability), so the printed fixed
// productions and the card's own basic land type's intrinsic ability precede
// the granted types' intrinsic abilities, and the choice-shaped ability is
// appended to the unit after them, once — each colour its own alternative
// even when an intrinsic ability already makes that colour, and each such
// step sorting by the printed ability it belongs to (pay.CompareSteps), not
// by the intrinsic that makes the same colour.
//
// No pool carries the shape beside Leyline of the Guildpact, and the kernel
// cannot reach it in a real game (no pool admits the card beside Leyline), so the census is probed on
// the planner unit directly, the way the kernel would replay it once a
// mixed-set pool admits the card.
func TestPlanLimitChoiceCensusOrder(t *testing.T) {
	set := cardsettest.Set(t, "hybridn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lib := CardID(0xffff)
	leyline := CardID(0xffff)
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Plains":
			lib = CardID(c)
		case "Leyline of the Guildpact":
			leyline = CardID(c)
		}
	}
	if lib == 0xffff || leyline == 0xffff {
		t.Fatal("the hybridn table lacks Plains or Leyline of the Guildpact")
	}
	plains := -1
	for i, n := range tbl.Subtypes {
		if n == "Plains" {
			plains = i
		}
	}
	if plains < 0 {
		t.Fatal("the hybridn table lacks the Plains subtype")
	}
	// a Restless-Spire-shaped land ("{T}: Add {U} or {R}") and a typed one
	// (its own basic land type's intrinsic beside a choice that makes that
	// colour), the two shapes the census order separates
	spire := CardID(len(tbl.Cards))
	choicePlains := CardID(len(tbl.Cards) + 1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Choice Spire", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU | kop.ColR, ManaN: 1, AFlags: kop.AfManaChoice},
		}},
		kop.CardDef{Name: "Test Choice Plains", Types: kop.TLand,
			Subtypes: kop.SubMask{}.With(plains), Abilities: []kop.Ability{
				{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1},
				{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW | kop.ColR, ManaN: 1, AFlags: kop.AfManaChoice},
			}},
	)
	sc := newScenTableLib(t, tbl, lib, Config{})
	if !sc.k.gen.blgOn {
		t.Fatal("blgOn unset: Leyline's AddSub carries no basic land type")
	}
	sc.perm(leyline, 0)
	spO := sc.perm(spire, 0)
	plO := sc.perm(choicePlains, 0)
	if bm := sc.s.blgMask(sc.k, sc.slot(spO)); bm != kop.ColAny {
		t.Fatalf("the spire's granted mask %#x; want the Leyline board's %#x", bm, kop.ColAny)
	}
	for _, tt := range []struct {
		name string
		o    Obj
		want []lsAlt
	}{
		{"spire", spO, []lsAlt{
			{uint8(kop.W), -1}, {uint8(kop.U), -1}, {uint8(kop.B), -1}, {uint8(kop.R), -1}, {uint8(kop.G), -1},
			{uint8(kop.U), 0}, {uint8(kop.R), 0},
		}},
		{"choice plains", plO, []lsAlt{
			{uint8(kop.W), -1}, {uint8(kop.U), -1}, {uint8(kop.B), -1}, {uint8(kop.R), -1}, {uint8(kop.G), -1},
			{uint8(kop.W), 1}, {uint8(kop.R), 1},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var pk planner
			pk.types[0].mask = uint8(kop.ColAny) // the blgView union: the printed colours and the granted ones
			pk.units[0], pk.utyp[0], pk.nu = sc.slot(tt.o), 0, 1
			var alts [planLimitAlts]lsAlt
			n, ok := sc.s.unitAltsChoice(sc.k, &pk, 0, &alts, true)
			if !ok {
				t.Fatal("unitAltsChoice declined: no choice-shaped ability, or no granted type for it to follow")
			}
			if n != len(tt.want) {
				t.Fatalf("%d alternatives %v; want %d %v", n, alts[:n], len(tt.want), tt.want)
			}
			for i, w := range tt.want {
				if alts[i] != w {
					t.Fatalf("alternative %d is %+v; want %+v (all %v)", i, alts[i], w, alts[:n])
				}
			}
		})
	}
}
