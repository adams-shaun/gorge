package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The granted conspire (CR 702.78a): Raiding Schemes gives each noncreature
// spell its controller casts conspire. No card in the pool prints the keyword,
// so every conspired cast, tap election and copy is the grant's. The policy
// casts creatures first (the board the election taps), prefers the conspired
// cast, and counts the conspired casts and the copies that reach the stack.

var kconspireDeck = []cardset.Entry{
	{Name: "Raiding Schemes", Count: 5}, {Name: "Johann's Stopgap", Count: 5}, {Name: "Shock", Count: 1},
	{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Iridescent Tiger", Count: 4},
	{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Grizzly Bears", Count: 3},
	{Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 7},
}

// kconspirePay is the ETB mana creatures: gorge offers the conspired cast
// only against the floating pool (its planner prices no such cast), so the
// policy casts them first, as the stackkw grant lockstep does.
var kconspirePay = map[string]bool{"Burning-Tree Emissary": true, "Iridescent Tiger": true}

var kconspireCreature = map[string]bool{
	"Grizzly Bears": true, "Merfolk of the Pearl Trident": true,
}

func kconspirePolicy(seed uint64, cnt map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x636f6e73)) // "cons"
	seen := map[state.ObjID]bool{}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		for _, id := range c.G.E.G.Stack {
			if o := c.G.E.G.Obj(id); o != nil && o.IsCopy && !seen[id] {
				seen[id] = true
				cnt["copy"]++
			}
		}
		note := func(a Canon) Canon {
			if a.Kind() == ActCast {
				if o, ok := c.G.GorgeOption(a); ok && o.Mode == "conspired" {
					cnt["conspired"]++
					if refCardName(c.G, a.Src()) == "Johann's Stopgap" {
						cnt["conspired-bargain"]++ // a Bargain spell cast conspired is never priced bargained
					}
				}
			}
			if cp, ok := chooseCode("conspire"); ok && a.Kind() == ActChoose+CanonKind(cp) {
				cnt["conspire-pick"]++ // the election over three or more eligible creatures
			}
			return a
		}
		if c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && kconspirePay[refCardName(c.G, a.Src())] {
					return note(a)
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && kconspireCreature[refCardName(c.G, a.Src())] {
					return note(a)
				}
			}
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast && o.Mode == "conspired" && refCardName(c.G, a.Src()) == "Johann's Stopgap" {
					return note(a)
				}
			}
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast && o.Mode == "conspired" && prng.IntN(4) != 0 {
					return note(a)
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return note(buf[prng.IntN(len(buf))])
			}
		}
		return note(rnd.Pick(c))
	})
}

func runStackkwConspire(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("conspire")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kconspireDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		if strings.Contains(strings.Join(c.Faces[0].Keywords, " "), "Conspire") {
			t.Fatalf("%s prints conspire: the counts would not be the grant's", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck has %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(kconspireDeck, kconspireDeck)
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]*cards.Card{deck, deck}
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kconspire-a", "kconspire-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kconspirePolicy(seed, cnt), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

func TestStackkwConspireLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runStackkwConspire(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, granted plays %v", games, divs, stalls, cnt)
	for _, w := range []string{"conspired", "copy", "conspire-pick", "conspired-bargain"} {
		if cnt[w] < 1 || (w != "conspired-bargain" && cnt[w] < 5) {
			t.Errorf("only %d granted %s plays (want >= 5)", cnt[w], w)
		}
	}
}

// TestStackkwConspireMutation drops the copy trigger: the copies vanish from
// the kernel's stack, so the lockstep must diverge.
func TestStackkwConspireMutation(t *testing.T) {
	old := dirac.DebugConspireNoCopy
	dirac.DebugConspireNoCopy = true
	defer func() { dirac.DebugConspireNoCopy = old }()
	games, divs, _, _ := runStackkwConspire(t, 64, false)
	if divs == 0 {
		t.Fatalf("copies dropped in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
