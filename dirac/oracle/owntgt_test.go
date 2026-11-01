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

// The lockstep: a DealDamage whose NumDmg$ / DamageSource$
// read its own target (Targeted$CardPower, DamageSource$ Targeted: Wisecrack,
// Justice Strike, Repentance) and Defined$ ThisTargetedCard (Malamet Battle
// Glyph's counter condition). Raging Goblin (haste) lets Malamet's target
// have entered this turn; creatures of power 0-3 (Wall of Wood 0/3, Pillarfield Ox 2/4 survive
// their own damage, so a wrong amount shows) vary the damage.
// Self-Destruct stays refused (its "any other target" may be a player, which
// the kernel's Bite cannot hit): see dirac/kgen/lower_owntarget.go.
var kowntgtDecks = [2][]cardset.Entry{
	{
		{Name: "Wisecrack", Count: 4}, {Name: "Malamet Battle Glyph", Count: 4}, {Name: "Raging Goblin", Count: 5},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 1}, {Name: "Wall of Wood", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Justice Strike", Count: 4}, {Name: "Repentance", Count: 3}, {Name: "Wisecrack", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 3},
		{Name: "Pillarfield Ox", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}

// kowntgtCounts are what gorge's event log showed.
type kowntgtCounts struct {
	wisecrack, strike, malamet, counters int
}

func kowntgtScan(g *GorgeSide) kowntgtCounts {
	var c kowntgtCounts
	if g == nil {
		return c
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Resolve:
			o := g.E.G.Obj(ev.Obj)
			if o == nil || o.Ability != nil {
				break
			}
			switch chName(g.E.G, ev.Obj) {
			case "Wisecrack":
				c.wisecrack++
			case "Justice Strike", "Repentance":
				c.strike++
			case "Malamet Battle Glyph":
				c.malamet++
			}
		case events.CounterChange:
			if ev.Counter == "P1P1" && ev.Amount > 0 {
				c.counters++ // only Malamet's condition puts a counter
			}
		}
	}
	return c
}

// kowntgtRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func kowntgtRun(t *testing.T, seeds uint64) (tot kowntgtCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("owntgt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kowntgtDecks {
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
	tbl, _, _, err := set.Matchup(kowntgtDecks[0], kowntgtDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kowntgt-a", "kowntgt-b"}, decks, 1)
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
		c := kowntgtScan(gs)
		tot.wisecrack += c.wisecrack
		tot.strike += c.strike
		tot.malamet += c.malamet
		tot.counters += c.counters
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestOwntgtLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := kowntgtRun(t, 300)
	t.Logf("%d games; Wisecrack resolutions %d; Justice Strike/Repentance %d; Malamet Battle Glyph %d (counter put: %d); stalls %v",
		games, tot.wisecrack, tot.strike, tot.malamet, tot.counters, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.wisecrack < 20 {
		t.Errorf("Wisecrack resolutions %d, want at least 20", tot.wisecrack)
	}
	if tot.malamet < 20 || tot.counters < 5 {
		t.Errorf("Malamet resolutions %d, counters %d, want at least 20 and 5", tot.malamet, tot.counters)
	}
}

// TestOwntgtMutation: with every Bite / DealDamageFrom amount one too high
// the lockstep must diverge within 128 seeds.
func TestOwntgtMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDmgAmountPlusOne = true
	defer func() { dirac.DebugDmgAmountPlusOne = false }()
	_, games, div, _ := kowntgtRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugDmgAmountPlusOne", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
