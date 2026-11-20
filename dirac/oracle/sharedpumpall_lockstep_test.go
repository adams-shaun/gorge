package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The SharedKeywordsZone$ PumpAll lockstep (fixture sharedpumpall): Odric,
// Lunarch Marshal's beginning-of-combat pump and Concerted Effort's upkeep
// pump grant every keyword one of your creatures has. Each deck carries one
// creature per keyword the cards list, so a green game still exercises the
// conditional grant: a creature that does not print a keyword gains it only
// while another creature of its controller prints it, and no keyword absent
// from the decks is ever granted.
var sharedPumpAllDecks = [2][]cardset.Entry{
	{
		{Name: "Odric, Lunarch Marshal", Count: 2}, {Name: "Concerted Effort", Count: 2},
		{Name: "White Knight", Count: 3}, {Name: "Serra Angel", Count: 3},
		{Name: "Fencing Ace", Count: 3}, {Name: "Troll Ascetic", Count: 3},
		{Name: "River Boa", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Concerted Effort", Count: 2},
		{Name: "Bog Wraith", Count: 3}, {Name: "Dross Prowler", Count: 3},
		{Name: "Vampire Nighthawk", Count: 3}, {Name: "Kederekt Creeper", Count: 3},
		{Name: "Kavu Predator", Count: 2}, {Name: "Giant Spider", Count: 2},
		{Name: "Raging Goblin", Count: 3}, {Name: "Darksteel Myr", Count: 2},
		{Name: "Typhoid Rats", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	},
}

// sharedPumpAllPrinted is the union of the decks' printed keywords: the only
// keywords the pump can share from them.
func sharedPumpAllPrinted(gd [2][]*cards.Card) map[string]bool {
	out := map[string]bool{}
	for _, d := range gd {
		for _, c := range d {
			for _, f := range c.Faces {
				for _, k := range f.Keywords {
					out[k] = true
				}
			}
		}
	}
	return out
}

// sharedPumpAllGranted reports whether a creature at id carries a keyword it
// does not print, and whether that keyword is a Landwalk or Protection family
// spelling.
func sharedPumpAllGranted(ref *GorgeSide, id state.ObjID) (granted, family bool) {
	printed := map[string]bool{}
	for _, k := range ref.E.G.Obj(id).Face().Keywords {
		printed[k] = true
	}
	for _, k := range ref.E.Derived(id).Keywords {
		if printed[k] {
			continue
		}
		granted = true
		if strings.HasPrefix(k, "Landwalk:") || strings.HasPrefix(k, "Protection") {
			family = true
		}
	}
	return granted, family
}

func TestSharedPumpAllLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "sharedpumpall", sharedPumpAllDecks)
	tbl, _, _, err := set.Matchup(sharedPumpAllDecks[0], sharedPumpAllDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	printed := sharedPumpAllPrinted(gd)
	sharedSeeds, familySeeds := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sharedpumpall-a", "sharedpumpall-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sawShared, sawFamily := false, false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			for p := state.PlayerID(0); p < 2; p++ {
				hasSource := false
				for _, id := range g.Zone(state.ZBattlefield, p) {
					n := g.Obj(id).Face().Name
					if n == "Odric, Lunarch Marshal" || n == "Concerted Effort" {
						hasSource = true
					}
				}
				if !hasSource {
					continue
				}
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if !g.Obj(id).Face().IsCreature() {
						continue
					}
					granted, family := sharedPumpAllGranted(ref, id)
					if !granted {
						continue
					}
					for _, kw := range ref.E.Derived(id).Keywords {
						if !printed[kw] {
							t.Errorf("seed %d: creature %q gained %q, which no deck card prints", seed, g.Obj(id).Face().Name, kw)
						}
					}
					sawShared = true
					sawFamily = sawFamily || family
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-10, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %q\nker %q\nlast %s\nactions %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail,
				d.RefOptions, d.KerOptions, d.LastSourced, actionStrings(res.Actions[lo:d.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if sawShared {
			sharedSeeds++
		}
		if sawFamily {
			familySeeds++
		}
	}
	if sharedSeeds == 0 {
		t.Fatal("no seed granted a shared keyword")
	}
	if familySeeds == 0 {
		t.Fatal("no seed shared a Landwalk or Protection spelling")
	}
	t.Logf("20 seeds: %d granted a shared keyword, %d shared a Landwalk or Protection", sharedSeeds, familySeeds)
}
