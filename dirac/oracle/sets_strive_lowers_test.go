package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSetsStriveLowers: Strive (CR 702.52a, "this spell costs <cost> more to
// cast for each target beyond the first"; CM2's Solidarity of Heroes and the
// corpus' other Strive spells) lowers as the self RaisePerTarget raise
// (kgen lower_strive.go), in lockstep with gorge's foldStriveCost.
//
// Precondition: gorge's games cast Strive spells with two or more targets, so
// the per-target charge is paid (counted from the stack below). The mutation
// drops the charge from every Strive spell's raise and must diverge.
func TestSetsStriveLowers(t *testing.T) {
	t.Parallel()
	t.Run("lockstep", func(t *testing.T) {
		t.Parallel()
		st := map[string]int{}
		pol := func(seed uint64) Policy { return setsStriveProbe(castFirstPolicy(seed), st) }
		if div := lockstepRunPolicy(t, "setsstrive", setsStriveDecks, 150, true, nil, nil, pol); div != 0 {
			return // each divergence is already a test error
		}
		multi := 0
		for _, n := range st {
			multi += n
		}
		t.Logf("Strive casts with two or more targets: %d %v", multi, st)
		if multi < 10 || len(st) < 16 {
			t.Fatalf("Strive casts with two or more targets: %d over %d cards, want at least 10 over all 16: the pool does not exercise it", multi, len(st))
		}
	})
	t.Run("mutation", func(t *testing.T) {
		t.Parallel()
		mutate := func(tbl *kop.Table) {
			n := 0
			for i := range tbl.Cards {
				cd := &tbl.Cards[i]
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.StFlags&kop.StRaisePerTarget != 0 {
						cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
						cd.Abilities[j].Static.A, cd.Abilities[j].Static.B = 0, 0 // the per-target charge dropped
						n++
					}
				}
			}
			if n < 16 {
				t.Fatalf("%d Strive raises in the table, want 16", n)
			}
		}
		pol := func(seed uint64) Policy { return castFirstPolicy(seed) }
		if div := lockstepRunPolicy(t, "setsstrive", setsStriveDecks, 60, false, mutate, nil, pol); div == 0 {
			t.Fatal("the Strive charge dropped from the kernel's raise, yet no divergence")
		}
	})
}

// setsStriveProbe wraps base and counts, when gorge leads, the Strive spells
// on the stack with two or more targets, by name (each object once).
func setsStriveProbe(base Policy, st map[string]int) Policy {
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZStack || o.Face() == nil || len(o.Targets) < 2 || seen[o.ID] {
				continue
			}
			if _, ok := o.Face().KeywordParam("Strive"); ok {
				seen[o.ID] = true
				st[o.Face().Name]++
			}
		}
		return a
	})
}

// setsStriveDecks: the sixteen Strive spells the lowering lets through, two
// of each, beside the creatures they target (Pacifism and Ornithopter for
// Consign to Dust and Kiora's Dismissal) and lands enough to pay for a
// second and third target.
var setsStriveDecks = [2][]cardset.Entry{
	{
		{Name: "Ajani's Presence", Count: 2}, {Name: "Blinding Flare", Count: 2}, {Name: "Desperate Stand", Count: 2},
		{Name: "Harness by Force", Count: 2}, {Name: "Launch the Fleet", Count: 2}, {Name: "Phalanx Formation", Count: 2},
		{Name: "Rouse the Mob", Count: 2}, {Name: "Twinflame", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Raging Goblin", Count: 1},
		{Name: "Pacifism", Count: 2}, {Name: "Ornithopter", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Aerial Formation", Count: 2}, {Name: "Colossal Heroics", Count: 2}, {Name: "Consign to Dust", Count: 2},
		{Name: "Cruel Feeding", Count: 2}, {Name: "Hour of Need", Count: 2}, {Name: "Kiora's Dismissal", Count: 2},
		{Name: "Nature's Panoply", Count: 2}, {Name: "Solidarity of Heroes", Count: 2},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 4},
	},
}
