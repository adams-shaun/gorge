package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestComboAnyPickKeepsOneColourEntries: removeCA must take a picked
// Combo Any source's units out with takeWide, not remove: a gap source added
// no one-colour entry (availCA only adds units), so removeWide's
// dropOneColour would forget the one entry of an unrelated "two mana of any
// one color" grant added into the same 0x1f group (ColAny), and the grant's
// remaining units would read as independent any-colour mana — maxX prices
// them more permissively than gorge's planner, which keeps the chooser's
// constraint.
func TestComboAnyPickKeepsOneColourEntries(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (prepareGen)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Combo Twin", Types: kop.TCreature,
		Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			AFlags:     kop.AfManaChoice,
			ManaColors: kop.ColAny,
			ManaN:      2,
			Cost:       kop.Cost{Flags: kop.CostTap},
		}},
	})
	sc := newScenTable(t, &tbl, Config{})
	sl := sc.slot(sc.perm(CardID(len(tbl.Cards)-1), 0))
	if !sc.k.gen.anyComboAny || !sc.s.caGaps(sc.k, 0).has(sl) {
		t.Fatalf("precondition: anyComboAny=%v, gap live=%v", sc.k.gen.anyComboAny, sc.s.caGaps(sc.k, 0).has(sl))
	}
	var a avail
	a.addUnits(0x1f, 2)      // the grant's group units (mana_grantcost.go: mask ColAny)
	sc.s.addOneColour(&a, 2) // the grant's one entry: 2 mana of any ONE colour
	a.addUnits(0x1f, 2)      // availCA's units for the Combo Any gap source
	sc.s.removeCA(sc.k, &a, sl)
	n := 0
	for i := 0; i < a.n; i++ {
		if a.mask[i] == 0x1f {
			n += int(a.cnt[i])
		}
	}
	if n != 2 {
		t.Fatalf("after the pick the 0x1f group holds %d units, want the grant's 2", n)
	}
	if a.nOne != 1 || a.one[0] != 2 {
		t.Fatalf("one entries n=%d one[0]=%d, want the grant's (2, any one colour) kept", a.nOne, a.one[0])
	}
}
