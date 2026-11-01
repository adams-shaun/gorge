package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep (CR 120.10): constructed decks over the
// generated excessdmg pool.
//
//   - Orbital Plunge: DealDamage with ExcessSVar$ and no condition; the
//     Lander token is made when the 6 damage beats the creature's lethal;
//   - Razor Rings: the excess is gained as life;
//   - Overwhelming Victory: the excess is the +X/+0 of a PumpAll;
//   - Goblin Negotiation: the old Card.targetedBy spelling (regression).
//
// Creatures of toughness 1-6 give the resolutions both excess > 0 and excess 0.
// Rhino's Rampage (Fight) stays refused: see dirac/kgen/lower_excessdamage.go.
var kexcessdmgDecks = [2][]cardset.Entry{
	{
		{Name: "Orbital Plunge", Count: 4}, {Name: "Goblin Negotiation", Count: 3}, {Name: "Overwhelming Victory", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Colossal Dreadmaw", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Trained Armodon", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Razor Rings", Count: 4}, {Name: "Orbital Plunge", Count: 3}, {Name: "Elite Vanguard", Count: 5},
		{Name: "Savannah Lions", Count: 4}, {Name: "Colossal Dreadmaw", Count: 1}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// kedCounts are what gorge's event log showed.
type kedCounts struct {
	plunge     int // Orbital Plunge resolutions (6 damage to a creature)
	plungeExc  int // ... with excess: a Lander token
	rings      int // Razor Rings life gains
	goblinToks int
}

func kedScan(g *GorgeSide) kedCounts {
	var c kedCounts
	if g == nil {
		return c
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Damage:
			if ev.Obj != 0 && ev.Amount == 6 {
				c.plunge++
			}
		case events.LifeChange:
			if ev.Amount > 0 {
				c.rings++
			}
		case events.TokenCreate:
			switch ev.Text {
			case "c_a_lander_sac_search":
				c.plungeExc++
			case "r_1_1_goblin":
				c.goblinToks++
			}
		}
	}
	return c
}

// kedRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func kedRun(t *testing.T, seeds uint64) (tot kedCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("excessdmg")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kexcessdmgDecks {
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
	tbl, _, _, err := set.Matchup(kexcessdmgDecks[0], kexcessdmgDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ked-a", "ked-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := kedScan(gs)
		tot.plunge += c.plunge
		tot.plungeExc += c.plungeExc
		tot.rings += c.rings
		tot.goblinToks += c.goblinToks
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestExcessdmgLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := kedRun(t, 300)
	t.Logf("%d games; Orbital Plunge resolutions %d (excess>0: %d, excess 0: %d); Razor Rings life gains %d; Goblin Negotiation tokens %d; stalls %v",
		games, tot.plunge, tot.plungeExc, tot.plunge-tot.plungeExc, tot.rings, tot.goblinToks, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.plungeExc < 10 || tot.plunge-tot.plungeExc < 5 {
		t.Errorf("Orbital Plunge excess>0 %d, excess 0 %d, want at least 10 and 5", tot.plungeExc, tot.plunge-tot.plungeExc)
	}
	if tot.rings < 5 {
		t.Errorf("Razor Rings life gains %d, want at least 5", tot.rings)
	}
}

// TestExcessdmgMutation: with lethal read as 0 (excess = all the damage)
// the lockstep must diverge within 128 seeds.
func TestExcessdmgMutation(t *testing.T) {
	dirac.DebugExcessNoLethal = true
	defer func() { dirac.DebugExcessNoLethal = false }()
	_, games, div, _ := kedRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugExcessNoLethal", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
