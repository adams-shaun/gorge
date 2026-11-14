package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMBCStatics2Lowers: the Mystery Booster Commander statics-2 spelling
// family in lockstep over the mbcstatics2 fixture pool, one subtest per
// spelling this change lowers:
//
//   - secondsvar: an AlternativeCost static's CheckSecondSVar$ (Massacre,
//     Submerge, Deepwood Legate: "if an opponent controls a <land> and you
//     control a <land>, you may cast this spell without paying its mana
//     cost"; gorge alternativeCostScopeOK reads CheckSVar$ and
//     CheckSecondSVar$ both). The caster's lands are Swamps and Islands and
//     the other side's Plains and Forests, so a game meets one half, both or
//     neither;
//   - canthave: a Continuous static's CantHaveKeyword$ (Archetype of
//     Imagination, Archetype of Courage: "creatures your opponents control
//     lose flying and can't have or gain flying"; gorge chars.derive's final
//     layer-6 filter) against the other side's flyers and Jump's "gains
//     flying until end of turn".
//
// A refused card would not build the pool, so the clean run fails without
// the lowering. Each subtest then mutates the table to drop what the
// lowering added (the second condition of the alternative cost; the
// Override's CantHaveKw prohibition) and requires some seed to diverge, so
// the pool reaches the read the lowering made.
func TestMBCStatics2Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decks  [2][]cardset.Entry
		seeds  uint64
		mutate func(*testing.T, *kop.Table)
	}{
		{"secondsvar", mbcSecondSVarDecks, 200, dropAltSecondCond},
		{"canthave", mbcCantHaveDecks, 200, dropCantHaveKw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if div := lockstepRun(t, "mbcstatics2", tc.decks, tc.seeds, true, nil); div != 0 {
				return // each divergence is already a test error
			}
			hit := false
			n := lockstepRun(t, "mbcstatics2", tc.decks, tc.seeds, false, func(tbl *kop.Table) {
				tc.mutate(t, tbl)
				hit = true
			})
			if !hit {
				t.Fatal("the mutation never ran")
			}
			if n == 0 {
				t.Fatalf("%s: no seed diverged with the lowered read dropped: the pool does not exercise it", tc.name)
			}
		})
	}
}

// dropAltSecondCond keeps only the first half of every alternative cost's
// two-part condition (CheckSVar$ AND CheckSecondSVar$).
func dropAltSecondCond(t *testing.T, tbl *kop.Table) {
	n := 0
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			a := &tbl.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAlternativeCost || a.Static.Cond2 == 0 {
				continue
			}
			if c := tbl.Conds[a.Static.Cond2]; c.Kind == kop.CondAnd {
				a.Static.Cond2 = c.Sub[0]
				n++
			}
		}
	}
	if n != 3 {
		t.Fatalf("%d two-part alternative-cost conditions, want 3 (Massacre, Submerge, Deepwood Legate)", n)
	}
}

// dropCantHaveKw clears every Override's CantHaveKeyword$ prohibition (the
// RemoveKeyword$ beside it stays).
func dropCantHaveKw(t *testing.T, tbl *kop.Table) {
	n := 0
	for i := range tbl.Overrides {
		if tbl.Overrides[i].CantHaveKw != 0 {
			if tbl.Overrides[i].RemoveKw&tbl.Overrides[i].CantHaveKw != tbl.Overrides[i].CantHaveKw {
				t.Fatalf("override %d prohibits %b without removing it", i, tbl.Overrides[i].CantHaveKw)
			}
			tbl.Overrides[i].CantHaveKw = 0
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d CantHaveKeyword overrides, want 2 (Archetype of Imagination, Archetype of Courage)", n)
	}
}

// mbcSecondSVarDecks: the free casts against Plains and Forests. The caster
// splits Swamps and Islands (Massacre and Deepwood Legate want a Swamp,
// Submerge an Island) and the other side Plains and Forests (Massacre wants
// a Plains, Submerge and Deepwood Legate a Forest).
var mbcSecondSVarDecks = [2][]cardset.Entry{
	{
		{Name: "Massacre", Count: 6}, {Name: "Submerge", Count: 6}, {Name: "Deepwood Legate", Count: 6},
		{Name: "Storm Crow", Count: 4}, {Name: "Swamp", Count: 9}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Savannah Lions", Count: 10},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	},
}

// mbcCantHaveDecks: the Archetypes against flyers and Jump, which the other
// side casts on its own creatures (gaining flying the Archetype forbids) and
// on the caster's.
var mbcCantHaveDecks = [2][]cardset.Entry{
	{
		{Name: "Archetype of Imagination", Count: 7}, {Name: "Archetype of Courage", Count: 7},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 11}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Storm Crow", Count: 7}, {Name: "Wind Drake", Count: 5}, {Name: "Jump", Count: 8},
		{Name: "Savannah Lions", Count: 4}, {Name: "Island", Count: 10}, {Name: "Plains", Count: 6},
	},
}
