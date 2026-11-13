package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copy exception "except it isn't legendary" lockstep:
// CopyPermanent NonLegendary$ True (CR 707.9b). Ember Island Production
// (a charm), Impostor Syndrome (a combat damage trigger) and The Notary
// Hobbits (two copies on entering) make tokens that copy a legendary creature
// without Legendary; the legend rule (CR 704.5j) must leave the copy beside
// its legendary original (or beside another such copy).

var kcopynonlegDecks = [2][]cardset.Entry{
	{
		{Name: "Ember Island Production", Count: 3}, {Name: "Impostor Syndrome", Count: 2},
		{Name: "Black Widow, Natasha Romanoff", Count: 3}, {Name: "Tyrox, Saurid Tyrant", Count: 3},
		{Name: "Happy Hogan, Dauntless Driver", Count: 3}, {Name: "Caelorna, Coral Tyrant", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "The Notary Hobbits", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Jasmine Boreal", Count: 3}, {Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 5},
		{Name: "Swords to Plowshares", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 7},
	},
}

// kcopynonlegLegends are the pool's legendary creatures.
var kcopynonlegLegends = map[string]bool{
	"Black Widow, Natasha Romanoff": true, "Tyrox, Saurid Tyrant": true, "Happy Hogan, Dauntless Driver": true,
	"Caelorna, Coral Tyrant": true, "The Notary Hobbits": true, "Isamaru, Hound of Konda": true, "Jasmine Boreal": true,
}

// kcopynonlegCounts counts what the games reach.
type kcopynonlegCounts struct {
	legCopies int // nonlegendary tokens copying a legendary creature
	coexist   int // games where such a token shared a battlefield with a same-named permanent of its controller
}

// kcopynonlegPolicy is the random policy that watches the battlefield at each
// decision (after the state-based actions): a token named like a legendary
// creature is a NonLegendary$ copy of it (every copy effect in the pool).
func kcopynonlegPolicy(seed uint64, st *kcopynonlegCounts) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	coexisted := false
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for p := state.PlayerID(0); p < 2; p++ {
				ids := g.Zone(state.ZBattlefield, p)
				for _, id := range ids {
					ob := g.Obj(id)
					if ob == nil || !ob.IsToken || ob.Face() == nil || !kcopynonlegLegends[ob.Face().Name] {
						continue
					}
					if !seen[id] {
						seen[id] = true
						st.legCopies++
					}
					if coexisted {
						continue
					}
					for _, oid := range ids {
						if oid == id {
							continue
						}
						if o := g.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == ob.Face().Name {
							coexisted = true
							st.coexist++
							break
						}
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runCopynonleg(t *testing.T, seeds int, mutate func(), reset func()) (games, divs, stalls, firstDiv int, pc kcopynonlegCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copynonleg")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
		defer reset()
	}
	var decks [2][]*cards.Card
	for s, d := range kcopynonlegDecks {
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
	tbl, _, _, err := set.Matchup(kcopynonlegDecks[0], kcopynonlegDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcopynonleg-a", "kcopynonleg-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcopynonlegPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if firstDiv == 0 {
				firstDiv = int(seed)
			}
			if mutate == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, firstDiv, pc
}

func TestCopynonlegLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, _, pc := runCopynonleg(t, 256, nil, nil)
	t.Logf("%d games, %d divergences, %d stalls, nonlegendary copies of a legendary creature %d, games with a copy beside a same-named permanent %d",
		games, divs, stalls, pc.legCopies, pc.coexist)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if pc.legCopies < 20 {
		t.Errorf("only %d NonLegendary copies of a legendary creature (need 20)", pc.legCopies)
	}
	if pc.coexist < 10 {
		t.Errorf("only %d games with a nonlegendary copy beside its original or another copy (need 10)", pc.coexist)
	}
}

// TestCopynonlegMutation keeps the copies legendary
// (dirac.DebugCopyNonLegKeep): the kernel's legend rule then kills them and
// the games must diverge within 64 seeds.
func TestCopynonlegMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, first, _ := runCopynonleg(t, 64,
		func() { dirac.DebugCopyNonLegKeep = true },
		func() { dirac.DebugCopyNonLegKeep = false })
	if divs == 0 {
		t.Fatalf("mutation kept the copies legendary in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games (first seed %d)", divs, games, first)
}
