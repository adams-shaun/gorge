package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The announced life-cost lockstep: "as an additional cost,
// pay X life" (Fix What's Broken, Vicious Rivalry: SVar:X:Count$xPaid, no
// {X} mana) and Krumar Initiate's {X}{B},{T}, Pay X life (the {X} mana and
// the life share one X). gorge poses the X ask for a LifeX cost part
// (xAsk, cast_subcounter.go), bounded by the payer's life; the life is one
// LifeChange of -x at the settle. The expected answer at every decision is
// gorge's.
var kpaylifexDecks = [2][]cardset.Entry{
	{
		{Name: "Vicious Rivalry", Count: 4}, {Name: "Krumar Initiate", Count: 4},
		{Name: "Midnight Tilling", Count: 2}, {Name: "Cache Grab", Count: 1}, {Name: "Liliana's Indignation", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Memnite", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Basal Thrull", Count: 3}, {Name: "Alpha Myr", Count: 1}, {Name: "Nether Horror", Count: 1},
		{Name: "Broodhunter Wurm", Count: 1}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Fix What's Broken", Count: 4}, {Name: "Krumar Initiate", Count: 4},
		{Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 2}, {Name: "Great Hart", Count: 1},
		{Name: "Gilded Sentinel", Count: 1}, {Name: "Canal Monitor", Count: 1}, {Name: "Memnite", Count: 1}, {Name: "Liliana's Indignation", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Alpha Myr", Count: 1}, {Name: "Basal Thrull", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
	},
}

// kpaylifexTally counts the X asks the lockstep poses, by card.
type kpaylifexTally struct {
	x1, x0   map[string]int // announcements with X >= 1 / X = 0
	lifeCap  int            // announced-only asks whose cap is the payer's life
	krumarLC int            // Krumar asks whose cap the life (not the mana) set
}

// kpaylifexPolicy is a random policy with three biases, so the rare shape
// is reached: gorge's planner prices no {X} activation, so Krumar's is
// offered only while the floating pool pays it. A random pick passes far
// more often than it casts or activates, so it takes an offered activation,
// casts Krumar and Liliana's Indignation (a self-mill, and an {X}{B} spell
// whose small X leaves mana floating) two times in three, and answers
// Liliana's X ask with its smallest value seven times in eight.
func kpaylifexPolicy(seed uint64, tl *kpaylifexTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6c696665)) // "life": the biases' own stream
	xcode, _ := chooseCode("x")
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		if c.D.Kind == DPriority {
			for _, o := range c.D.Options {
				switch o.Kind() {
				case ActActivate:
					a = o
				case ActCast:
					if g, ok := c.G.GorgeOption(o); ok && g.Kind == "cast" && r.IntN(3) != 0 {
						if n := c.G.E.G.Obj(g.Obj).Face().Name; n == "Krumar Initiate" || n == "Liliana's Indignation" {
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
			a = c.D.Options[0]
		}
		if name != "Fix What's Broken" && name != "Vicious Rivalry" && name != "Krumar Initiate" {
			return a
		}
		cap := 0
		for _, o := range c.D.Options {
			cap = max(cap, int(o.Mode()))
		}
		life := max(0, int(c.G.E.G.Players[pd.Player].Life))
		if cap == life {
			if name == "Krumar Initiate" {
				tl.krumarLC++
			} else {
				tl.lifeCap++
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

// runPaylifex plays seeds 1..seeds and returns the games, the divergences,
// the kernel capacity stalls and the tally; mutate sets DebugLifeXNoPay
// (the mutation must diverge).
func runPaylifex(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl kpaylifexTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("paylifex")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugLifeXNoPay
	dirac.DebugLifeXNoPay = mutate
	defer func() { dirac.DebugLifeXNoPay = old }()
	tl.x1, tl.x0 = map[string]int{}, map[string]int{}
	var decks [2][]*cards.Card
	for s, d := range kpaylifexDecks {
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
	tbl, _, _, err := set.Matchup(kpaylifexDecks[0], kpaylifexDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpaylifex-a", "kpaylifex-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kpaylifexPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
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

func TestPaylifexLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runPaylifex(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; announcements X>=1 %v, X=0 %v; life-capped asks: announced-only %d, Krumar %d",
		games, divs, stalls, tl.x1, tl.x0, tl.lifeCap, tl.krumarLC)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, n := range []string{"Fix What's Broken", "Vicious Rivalry"} {
		if tl.x1[n] < 20 || tl.x0[n] < 5 {
			t.Errorf("%s: X>=1 %d (want 20), X=0 %d (want 5)", n, tl.x1[n], tl.x0[n])
		}
	}
	if tl.x1["Krumar Initiate"] < 20 {
		t.Errorf("Krumar Initiate: X>=1 %d (want 20)", tl.x1["Krumar Initiate"])
	}
	if tl.lifeCap < 5 {
		t.Errorf("life-capped announced-only asks %d (want 5)", tl.lifeCap)
	}
}

// TestPaylifexMutation: DebugLifeXNoPay pays 0 for the announced life, so
// the controller's life total (and the lose-life events) diverge.
func TestPaylifexMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runPaylifex(t, 64, true)
	if divs == 0 {
		t.Fatalf("DebugLifeXNoPay: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
