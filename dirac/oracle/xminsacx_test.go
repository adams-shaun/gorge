package oracle

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The announced-X floor and the announced sacrifice lockstep: Lair of the
// Hydra ("X G XMin1") floors a mana {X} at 1 (Mind Grind's spell Cost$
// "XMin1 X U B" is not played: gorge offers its {X} at 0, kgen refuses it);
// Eliminate the Competition sacrifices X creatures (no {X} mana, no floor:
// X = 0 is a legal announcement); Ruthless Technomancer ("XMin1 2 B
// Sac<X/Artifact>") has both, so its X is bounded by the artifacts its
// controller has and floored at 1. gorge poses the ask (xAsk,
// cast_subcounter.go); the expected answer at every decision is gorge's.
var kxminsacxDecks = [2][]cardset.Entry{
	{
		{Name: "Ruthless Technomancer", Count: 4}, {Name: "Eliminate the Competition", Count: 4},
		{Name: "Ornithopter", Count: 6}, {Name: "Memnite", Count: 3}, {Name: "Alpha Myr", Count: 2}, {Name: "Gilded Sentinel", Count: 1},
		{Name: "Walking Corpse", Count: 2}, {Name: "Liliana's Indignation", Count: 4}, {Name: "Swamp", Count: 14},
	},
	{
		{Name: "Lair of the Hydra", Count: 4}, {Name: "Ruthless Technomancer", Count: 4}, {Name: "Eliminate the Competition", Count: 3},
		{Name: "Ornithopter", Count: 6}, {Name: "Memnite", Count: 3}, {Name: "Alpha Myr", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Basal Thrull", Count: 2}, {Name: "Liliana's Indignation", Count: 4}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
	},
}

// kxminsacxTally counts the X asks the lockstep poses, by card.
type kxminsacxTally struct {
	x1, x0   map[string]int // announcements with X >= 1 / X = 0
	zeroOffs map[string]int // asks of a floored card that offered X = 0
	techCap  int            // Technomancer asks whose option ceiling is its controller's artifact count
	techAsks int            // Technomancer asks
	offered  map[string]int // priority windows offering a non-mana activation, by card
}

// kxminsacxPolicy is a random policy biased toward the carriers: an offered
// activation is taken, the carriers are cast two times in three, and the X
// ask is answered with its largest value one time in three, its smallest
// otherwise, so both ends of the range are reached.
func kxminsacxPolicy(seed uint64, tl *kxminsacxTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x78736163)) // "xsac": the biases' own stream
	xcode, _ := chooseCode("x")
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		if c.D.Kind == DPriority {
			ability := false // an offered non-mana activation is taken over a mana one
			for _, o := range c.D.Options {
				switch o.Kind() {
				case ActActivate:
					if g, ok := c.G.GorgeOption(o); ok && g.Kind == "ability" {
						tl.offered[c.G.E.G.Obj(g.Obj).Face().Name]++
						a, ability = o, true
					} else if !ability {
						a = o
					}
				case ActCast:
					if g, ok := c.G.GorgeOption(o); ok && g.Kind == "cast" && r.IntN(3) != 0 {
						switch c.G.E.G.Obj(g.Obj).Face().Name {
						case "Lair of the Hydra", "Eliminate the Competition", "Ruthless Technomancer", "Liliana's Indignation":
							a = o
						}
					}
				}
			}
			return a
		}
		if c.D.Kind != DChoose || len(c.D.Options) == 0 || c.D.Options[0].Kind() != ActChoose+CanonKind(xcode) {
			return a
		}
		pd := c.G.Pending()
		if pd == nil {
			return a
		}
		name := c.G.E.G.Obj(pd.Source).Face().Name
		if name == "Liliana's Indignation" && r.IntN(8) != 0 {
			return c.D.Options[0] // its smallest X leaves mana floating
		}
		switch name {
		case "Ruthless Technomancer", "Eliminate the Competition", "Lair of the Hydra":
		default:
			return a
		}
		lo, hi := 1<<30, 0
		for _, o := range c.D.Options {
			lo, hi = min(lo, int(o.Mode())), max(hi, int(o.Mode()))
		}
		if r.IntN(3) == 0 || name == "Eliminate the Competition" && r.IntN(2) == 0 {
			a = c.D.Options[len(c.D.Options)-1]
		} else if r.IntN(2) == 0 {
			a = c.D.Options[0]
		}
		if name != "Eliminate the Competition" && lo == 0 {
			tl.zeroOffs[name]++
		}
		if name == "Ruthless Technomancer" {
			tl.techAsks++
			arts := 0
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, pd.Player) {
				if objectHasType(c.G.E.G.Obj(id), "Artifact") {
					arts++
				}
			}
			if hi == arts {
				tl.techCap++
			}
		}
		if a.Mode() >= 1 {
			tl.x1[name]++
		} else {
			tl.x0[name]++
		}
		return a
	})
}

// runXMinSacX plays seeds lo..hi and returns the games, the divergences, the
// kernel capacity stalls and the tally; mutate sets DebugXMinIgnored (the
// mutation must diverge).
func runXMinSacX(t *testing.T, lo, hi int, mutate bool) (games, divs, stalls int, tl kxminsacxTally) {
	t.Helper()
	old := dirac.DebugXMinIgnored
	dirac.DebugXMinIgnored = mutate
	defer func() { dirac.DebugXMinIgnored = old }()
	tl.x1, tl.x0, tl.zeroOffs, tl.offered = map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	// the seeds play as parallel chunks (seedChunks) under the flag above
	seedChunks(t, uint64(lo), uint64(hi), 150, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		g, d, s, part := xminsacxChunk(t, int(lo), int(hi), mutate)
		mu.Lock()
		defer mu.Unlock()
		games, divs, stalls = games+g, divs+d, stalls+s
		tl.techCap += part.techCap
		tl.techAsks += part.techAsks
		for _, mp := range [][2]map[string]int{{tl.x1, part.x1}, {tl.x0, part.x0}, {tl.zeroOffs, part.zeroOffs}, {tl.offered, part.offered}} {
			for k, v := range mp[1] {
				mp[0][k] += v
			}
		}
	})
	return games, divs, stalls, tl
}

// xminsacxChunk plays seeds lo..hi of runXMinSacX's games.
func xminsacxChunk(t *testing.T, lo, hi int, mutate bool) (games, divs, stalls int, tl kxminsacxTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("xminsacx")
	if err != nil {
		t.Fatal(err)
	}
	tl.x1, tl.x0, tl.zeroOffs, tl.offered = map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	var decks [2][]*cards.Card
	for s, d := range kxminsacxDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kxminsacxDecks[0], kxminsacxDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(lo); seed <= uint64(hi); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kxminsacx-a", "kxminsacx-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kxminsacxPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				from := d.At - 12
				if from < 0 {
					from = 0
				}
				dbg := ""
				for pl := state.PlayerID(0); pl < 2; pl++ {
					var bf, gy []string
					for _, id := range g.E.G.Zone(state.ZBattlefield, pl) {
						bf = append(bf, g.E.G.Obj(id).Face().Name)
					}
					for _, id := range g.E.G.Zone(state.ZGraveyard, pl) {
						gy = append(gy, g.E.G.Obj(id).Face().Name)
					}
					dbg += fmt.Sprintf(" p%d bf %v gy %v;", pl, bf, gy)
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s [%s]\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, dbg, d.RefOptions, d.KerOptions)
			}
		}
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return games, divs, stalls, tl
}

func TestXMinSacXLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip here, before the chunks (each would
	// skip alone and leave this test's totals empty).
	corpusDirOrSkip(t)
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runXMinSacX(t, 1, 1200, false)
	t.Logf("activations offered %v", tl.offered)
	t.Logf("%d games, %d divergences, %d stalls; announcements X>=1 %v, X=0 %v; floored asks offering X=0 %v; Technomancer asks %d, artifact-capped %d",
		games, divs, stalls, tl.x1, tl.x0, tl.zeroOffs, tl.techAsks, tl.techCap)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if n := tl.x1["Ruthless Technomancer"]; n < 10 {
		t.Errorf("Ruthless Technomancer: X>=1 %d (want 10)", n)
	}
	if n := tl.x1["Eliminate the Competition"]; n < 10 {
		t.Errorf("Eliminate the Competition: X>=1 %d (want 10)", n)
	}
	if n := tl.x0["Eliminate the Competition"]; n < 3 {
		t.Errorf("Eliminate the Competition: X=0 %d (want 3)", n)
	}
	if n := tl.x1["Lair of the Hydra"]; n < 10 {
		t.Errorf("Lair of the Hydra: X>=1 %d (want 10)", n)
	}
	if len(tl.zeroOffs) != 0 {
		t.Errorf("a floored card was offered X = 0: %v", tl.zeroOffs)
	}
	if tl.techCap < 5 {
		t.Errorf("Technomancer asks capped by the artifact count %d (want 5)", tl.techCap)
	}
}

// TestXMinSacXMutation: DebugXMinIgnored reads every floor as 0, so the X
// ask offers X = 0 (and an unpayable floored offer is made) where gorge
// offers none.
func TestXMinSacXMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runXMinSacX(t, 1, 64, true)
	if divs == 0 {
		t.Fatalf("DebugXMinIgnored: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
