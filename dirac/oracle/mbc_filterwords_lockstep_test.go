package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC filters-9 spelling family, lockstep against gorge: Kitesail
// Skirmisher's attack trigger targets another attacker attacking the same
// defender (attackingSame, kop.W7AttackingSame), Gilt-Leaf Winnower's enters
// trigger destroys a non-Elf whose power and toughness differ (!equalPT) and
// Master Symmetrist and Symmetry Matrix read equalPT on the attacking or
// entering creature (both are kop.PredPowerLTToughness with a PredN relation).
// The expected answer at every decision is gorge's.
var kmbcFilters9Decks = [2][]cardset.Entry{
	{
		{Name: "Kitesail Skirmisher", Count: 4}, {Name: "Gilt-Leaf Winnower", Count: 4}, {Name: "Master Symmetrist", Count: 3},
		{Name: "Symmetry Matrix", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Wind Drake", Count: 3},
		{Name: "Giant Spider", Count: 2}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Kitesail Skirmisher", Count: 3}, {Name: "Gilt-Leaf Winnower", Count: 4}, {Name: "Master Symmetrist", Count: 3},
		{Name: "Symmetry Matrix", Count: 3}, {Name: "Giant Spider", Count: 4}, {Name: "Elvish Warrior", Count: 4},
		{Name: "Birds of Paradise", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Island", Count: 5},
		{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 3},
	},
}

func TestMBCFilters9Lowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcfilters9", kmbcFilters9Decks, 400, "KMBCFILTERS9_SEEDS")
}

// TestMBCFilters9LockstepMutation: with the attackingSame read dropped, or
// the equalPT relation read as the plain power < toughness, some seed
// diverges, so the pool exercises each word.
func TestMBCFilters9LockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Filter) bool
	}{
		{"attackingSame", func(f *kop.Filter) bool {
			if f.Pred == kop.PredWord7 && f.PredN&0xff == kop.W7AttackingSame {
				f.Pred, f.PredN = kop.PredNone, 0
				return true
			}
			return false
		}},
		{"equalPT", func(f *kop.Filter) bool {
			if f.Pred == kop.PredPowerLTToughness && f.PredN == kop.PTRelEQ {
				f.PredN = kop.PTRelLT
				return true
			}
			return false
		}},
		{"!equalPT", func(f *kop.Filter) bool {
			if f.Pred == kop.PredPowerLTToughness && f.PredN == kop.PTRelNE {
				f.PredN = kop.PTRelLT
				return true
			}
			return false
		}},
	} {
		t.Run(m.name, func(t *testing.T) {
			var hit bool
			n := lockstepRun(t, "mbcfilters9", kmbcFilters9Decks, 400, false, func(tbl *kop.Table) {
				fs := append([]kop.Filter(nil), tbl.Filters...)
				for i := range fs {
					hit = m.fn(&fs[i]) || hit
				}
				tbl.Filters = fs
			})
			if !hit {
				t.Fatalf("the table carries no %s filter", m.name)
			}
			if n == 0 {
				t.Fatalf("no seed diverged with the %s read mutated", m.name)
			}
		})
	}
}
