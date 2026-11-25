package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// askColPotentialScen is the kfuzz board that over-offered Rona's Vortex: p0
// controls a colour-asking source (Nyx Lotus's {T}: choose a colour, add the
// devotion to it), three red sources and one blue, no black permanent, and
// holds a Kicker {2}{B} instant costing {U}. gorge's PotentialMana reads the
// AB$ ChooseColor head's own Mana params (no Produced$, no Amount$), one {C},
// so no potential play reaches the source's devotion mana and the kicked cast
// is not offered.
func askColPotentialScen(t *testing.T) (sc *scen, spell Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	dev := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	asker := add(kop.CardDef{Name: "Test Asker", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaV: dev, ManaAskColour: true}}})
	land := func(name string, col kop.ColorMask) CardID {
		return add(kop.CardDef{Name: name, Types: kop.TLand,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: col, ManaN: 1}}})
	}
	mountain := land("Test Mountain", 1<<kop.R)
	island := land("Test Island", 1<<kop.U)
	kicked := add(kop.CardDef{Name: "Test Vortex", Types: kop.TInstant,
		Cost:      kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1}},
		Keywords:  kop.KwKicker,
		Kicker:    kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.B: 1}},
		Abilities: []kop.Ability{{Kind: kop.AbSpell}}})
	sc = newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(asker, 0)
	for i := 0; i < 3; i++ {
		sc.perm(mountain, 0)
	}
	sc.perm(island, 0)
	return sc, sc.hand(kicked, 0)
}

// TestAskColourPotentialDoesNotOpenThePool: the colour-asking source's
// potential unit is the head's one {C} (gorge's addPotentialMana over a
// ChooseColor head), never an open any-colour production. Treating it as open
// makes the potential walk pay the kicker's {B} with a devotion of zero and
// offers a kicked cast the real census and gorge's planner never price.
func TestAskColourPotentialDoesNotOpenThePool(t *testing.T) {
	sc, spell := askColPotentialScen(t)
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	if !hasOption(&d, MakeAction(AkCast, TCode(spell), 0, 0)) {
		t.Fatalf("precondition: the plain {U} cast is not offered: %v", d.Options[:d.N])
	}
	if hasOption(&d, MakeAction(AkCast, TCode(spell), 0, 1)) {
		t.Fatalf("the kicked {2}{U}{B} cast is offered with no black devotion: the colour-asking source's head is an open any-colour production, not one {C}: %v", d.Options[:d.N])
	}
}
