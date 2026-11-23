package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// feeSpareScen is vanilla's table plus the pin's cards, hand-written (the
// synthetic pools are the lockstep oracle's and kgen's to read): Urn of
// Godfire ("{2}: Add one mana of any color", a fee-only filter mana ability),
// Mox Sapphire, Grim Physician ({B}), Feral Ghoul ({2}{B}) and the Powerstone
// token — a {C} source restricted to artifact spells and to every non-spell
// payment (AfManaNonSpell, RestrictValid$ CantCastNonArtifactSpells, CR
// 106.6). The pips and sizes are the pin's only read of these cards; the
// shapes are the lowered ones of the tail-50 pool (the Urn FeeOnlyManaAb, the
// token's ManaRestrict filter over the stack).
func feeSpareScen(t *testing.T) (sc *scen, id map[string]CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact, Zone: kop.ZoneStack})
	artifactSpells := kop.FilterID(len(tbl.Filters) - 1)
	id = map[string]CardID{"Plains": v.Plains, "Swamp": v.Swamp}
	add := func(cd kop.CardDef) {
		id[cd.Name] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, cd)
	}
	add(kop.CardDef{Name: "Urn of Godfire", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 2}},
			ManaColors: kop.ColAny, ManaN: 1}}})
	add(kop.CardDef{Name: "Mox Sapphire", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColU, ManaN: 1}}})
	add(kop.CardDef{Name: "Grim Physician", Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{2: 1}},
		Types: kop.TCreature, Colors: kop.ColB, Power: 1, Toughness: 2})
	add(kop.CardDef{Name: "Feral Ghoul", Cost: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{2: 1}},
		Types: kop.TCreature, Colors: kop.ColB, Power: 2, Toughness: 2})
	add(kop.CardDef{Name: "Powerstone Token", Types: kop.TArtifact | kop.TToken,
		Abilities: []kop.Ability{{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell,
			Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1, ManaRestrict: artifactSpells}}})
	sc = newScenTable(t, &tbl, Config{NoElide: true})
	return sc, id
}

// CR 106.6, 605.3a: a Powerstone's {C} can't pay a nonartifact spell but does
// pay the {2} of Urn of Godfire's "{2}: Add one mana of any color", an activated
// mana ability, and that mana then pays the spell. gorge's bound counts it
// (potentialFeeSpare); the kernel dropped the cast: Grim Physician ({B}, decision
// 745) at the potential walk, Feral Ghoul ({2}{B}, decision 801) in the filter
// search.
func TestFeeSparePowerstoneFundsUrnForSpell(t *testing.T) {
	for _, c := range []struct {
		spell string
		board []string
		want  bool
	}{
		{"Grim Physician", []string{"Plains", "Plains"}, true}, // the control: unrestricted mana pays the fee
		{"Grim Physician", []string{"Powerstone Token", "Plains"}, true},
		{"Grim Physician", []string{"Powerstone Token", "Powerstone Token"}, true},
		{"Grim Physician", []string{"Powerstone Token", "Mox Sapphire"}, true},
		{"Grim Physician", []string{"Powerstone Token"}, false}, // one {C} leaves the {2} fee short
		// decision 801: the Powerstones' {C} pay the fee, the Urn's mana, the
		// Mox and the Swamp pay {2}{B}.
		{"Feral Ghoul", []string{"Swamp", "Mox Sapphire", "Powerstone Token", "Powerstone Token"}, true},
		{"Feral Ghoul", []string{"Swamp", "Powerstone Token", "Powerstone Token"}, false}, // the Urn and the Swamp make two mana for {2}{B}
		{"Feral Ghoul", []string{"Swamp", "Mox Sapphire", "Powerstone Token"}, false},     // one {C} leaves the fee short
	} {
		sc, id := feeSpareScen(t)
		for _, n := range c.board {
			sc.perm(id[n], 0)
		}
		sc.perm(id["Urn of Godfire"], 0)
		sp := sc.hand(id[c.spell], 0)
		d := sc.s.Step()
		if got := castOffered(&d, sp); got != c.want {
			t.Errorf("board %v + Urn: %s offered=%v, want %v", c.board, c.spell, got, c.want)
		}
	}
}
