package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The CastWithFlash ValidSA$ tail lockstep (gorge spellMatchesValidSA,
// flashXCap): Silver Scrutiny's own `ValidSA$ Spell.XCostLE3` grant lets it
// be cast at instant speed with X at most 3 (the kernel's flashXCap caps the
// X menu as gorge's xAsk does), and Blur of Heroism's `ValidSA$ Spell.Hero`
// grant gives its controller's Hero spells flash and nothing else.

var cwfXCostDecks = [2][]cardset.Entry{
	{ // A: W/U Heroes under Blur of Heroism, with Silver Scrutiny
		{Name: "Blur of Heroism", Count: 4}, {Name: "Amateur Hero", Count: 4}, {Name: "Agent of Atlas", Count: 4},
		{Name: "Silver Scrutiny", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Cancel", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 10},
	},
	{ // B: U/W Silver Scrutiny and non-Heroes
		{Name: "Silver Scrutiny", Count: 4}, {Name: "Blur of Heroism", Count: 2}, {Name: "Agent of Atlas", Count: 3},
		{Name: "Glory Seeker", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Cancel", Count: 3},
		{Name: "Island", Count: 11}, {Name: "Plains", Count: 10},
	},
}

type cwfXCostStats struct {
	scrutiny int // off-sorcery-timing Silver Scrutiny casts taken
	hero     int // off-sorcery-timing Hero creature casts taken
	nonHero  int // off-sorcery-timing non-Hero creature casts offered (must stay 0)
}

// cwfXCostProbe wraps a seeded random policy biased to take an off-timing
// cast when one is offered, and counts them.
type cwfXCostProbe struct {
	st   *cwfXCostStats
	base Policy
	r    *rand.Rand
}

func faceHasType(f *cards.Face, t string) bool {
	for _, x := range f.Types {
		if x == t {
			return true
		}
	}
	return false
}

func (p *cwfXCostProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	d := c.G.Pending()
	if d == nil || d.Kind != "priority" {
		return a
	}
	g := c.G.E.G
	if g.Step.IsMain() && g.Active == g.Priority && len(g.Stack) == 0 {
		return a
	}
	var offered []Canon
	for _, x := range c.D.Options {
		if x.Kind() != ActCast {
			continue
		}
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		ob := g.Obj(o.Obj)
		if ob == nil || ob.Face() == nil {
			continue
		}
		f := ob.Face()
		switch {
		case f.Name == "Silver Scrutiny", faceHasType(f, "Creature") && faceHasType(f, "Hero"):
			offered = append(offered, x)
		case faceHasType(f, "Creature"):
			p.st.nonHero++
		}
	}
	if len(offered) > 0 && p.r.IntN(4) != 0 {
		a = offered[p.r.IntN(len(offered))]
	}
	if a.Kind() == ActCast {
		for _, x := range offered {
			if x != a {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && g.Obj(o.Obj).Face().Name == "Silver Scrutiny" {
				p.st.scrutiny++
			} else {
				p.st.hero++
			}
		}
	}
	return a
}

func TestCwfXCostSubtypeLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cwfxcost")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range cwfXCostDecks {
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
	tbl, _, _, err := set.Matchup(cwfXCostDecks[0], cwfXCostDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st cwfXCostStats
	stalls := 0
	const seeds = 128
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cwfxcost-a", "cwfxcost-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &cwfXCostProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x63776678))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %+v, %d stalls", seeds, st, stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.scrutiny < 3 || st.hero < 3 {
		t.Errorf("off-timing casts: Silver Scrutiny %d, Hero %d; want at least 3 each", st.scrutiny, st.hero)
	}
	if st.nonHero != 0 {
		t.Errorf("a non-Hero creature was offered off-timing %d times", st.nonHero)
	}
}
