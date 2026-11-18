package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Rayami, First of the Fallen lockstep: a dying nontoken creature is
// exiled with a blood counter, and Rayami shares the printed keywords of the
// exiled creature cards that carry one (SharedKeywordsZone$ Exile). Each half
// is proved to have happened: a creature card with a printed keyword sits in
// exile with a BLOOD counter while Rayami is in play, and Rayami's derived
// keywords then hold it (a run that never lowers the static grants nothing,
// one that grants without the gate grants it with nothing exiled).
var rayamiDecks = [2][]cardset.Entry{
	{
		{Name: "Rayami, First of the Fallen", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Typhoid Rats", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Wind Drake", Count: 4}, {Name: "Typhoid Rats", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Doom Blade", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 6},
	},
}

func TestRayamiLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "rayami", rayamiDecks)
	tbl, _, _, err := set.Matchup(rayamiDecks[0], rayamiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	exiled, shared, noneShared := 0, 0, 0
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rayami-a", "rayami-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sawExiled, sawShared := false, false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			var carried []string // the keywords of the BLOOD-counter exiled creature cards
			for p := state.PlayerID(0); p < 2; p++ {
				for _, o := range g.Zone(state.ZExile, p) {
					ob := g.Obj(o)
					if ob.Counter("BLOOD") > 0 && ob.Face().IsCreature() {
						carried = append(carried, ob.Face().Keywords...)
					}
				}
			}
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if g.Obj(id).Face().Name != "Rayami, First of the Fallen" {
						continue
					}
					kws := ref.E.Derived(id).Keywords
					if len(carried) == 0 {
						if slices.Contains(kws, "Flying") || slices.Contains(kws, "Deathtouch") {
							noneShared++
						}
						continue
					}
					sawExiled = true
					if (slices.Contains(carried, "Flying") && slices.Contains(kws, "Flying")) ||
						(slices.Contains(carried, "Deathtouch") && slices.Contains(kws, "Deathtouch")) {
						sawShared = true
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		if sawExiled {
			exiled++
		}
		if sawShared {
			shared++
		}
	}
	if noneShared != 0 {
		t.Fatalf("Rayami had flying or deathtouch with no blood-counter creature in exile %d times", noneShared)
	}
	if exiled == 0 {
		t.Fatal("no seed reached a blood-counter creature in exile beside Rayami")
	}
	if shared == 0 {
		t.Fatal("no seed shared a printed keyword from the exile to Rayami")
	}
	t.Logf("60 seeds: %d reached a blood-counter exiled creature, %d shared its keyword", exiled, shared)
}
