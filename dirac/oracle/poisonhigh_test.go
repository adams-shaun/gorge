package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The opponents'-highest-poison lockstep: the ONE
// Corrupted carriers -- static keyword grants, an activated draw, an
// activated tap, attack and enter triggers, a cost reduction, an aura and a
// sacrifice-unless enter trigger -- beside infect and toxic attackers, so
// poison crosses the three-counter threshold and the kernel's cold
// ExHighestOppPoison reader decides at every decision the way gorge's
// playerCountExtreme does. The frozen FDN and BLB pair pools exercise none
// of these arms.

var poisonhighDecks = [][2][]cardset.Entry{{
	// the static keyword grants and the aura, against a poison-free deck
	{
		{Name: "Blighted Agent", Count: 4}, {Name: "Glistener Elf", Count: 4}, {Name: "Bilious Skulldweller", Count: 4},
		{Name: "Apostle of Invasion", Count: 3}, {Name: "Bonepicker Skirge", Count: 3}, {Name: "Incisor Glider", Count: 3},
		{Name: "Zealot's Conviction", Count: 2}, {Name: "Forest", Count: 7}, {Name: "Plains", Count: 4},
		{Name: "Island", Count: 3}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Forest", Count: 7},
		{Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Plains", Count: 3},
	},
}, {
	// the activated and triggered surfaces
	{
		{Name: "Blighted Agent", Count: 4}, {Name: "Glistener Elf", Count: 4}, {Name: "Chittering Skitterling", Count: 3},
		{Name: "Sinew Dancer", Count: 3}, {Name: "Ravenous Necrotitan", Count: 2},
		{Name: "Vivisection Evangelist", Count: 2}, {Name: "Distorted Curiosity", Count: 2}, {Name: "Walking Corpse", Count: 1},
		{Name: "Island", Count: 4}, {Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Forest", Count: 7},
		{Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Plains", Count: 3},
	},
}, {
	// the mirror: both sides poison each other, so the controller's own
	// count differs from the opponents' highest late in a game
	{
		{Name: "Blighted Agent", Count: 4}, {Name: "Glistener Elf", Count: 4}, {Name: "Apostle of Invasion", Count: 2},
		{Name: "Bonepicker Skirge", Count: 2}, {Name: "Chittering Skitterling", Count: 2}, {Name: "Sinew Dancer", Count: 2},
		{Name: "Incisor Glider", Count: 2}, {Name: "Zealot's Conviction", Count: 2},
		{Name: "Ravenous Necrotitan", Count: 2}, {Name: "Vivisection Evangelist", Count: 2}, {Name: "Distorted Curiosity", Count: 2},
		{Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 2},
		{Name: "Mountain", Count: 2},
	},
	{
		{Name: "Blighted Agent", Count: 4}, {Name: "Glistener Elf", Count: 4}, {Name: "Apostle of Invasion", Count: 2},
		{Name: "Bonepicker Skirge", Count: 2}, {Name: "Chittering Skitterling", Count: 2}, {Name: "Sinew Dancer", Count: 2},
		{Name: "Incisor Glider", Count: 2}, {Name: "Zealot's Conviction", Count: 2},
		{Name: "Ravenous Necrotitan", Count: 2}, {Name: "Vivisection Evangelist", Count: 2}, {Name: "Distorted Curiosity", Count: 2},
		{Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 2},
		{Name: "Mountain", Count: 2},
	},
}}

// poisonhighCarriers names the cards whose casts and activations the test
// counts (each must be played at least once).
var poisonhighCarriers = []string{"Apostle of Invasion", "Bonepicker Skirge", "Chittering Skitterling",
	"Distorted Curiosity", "Incisor Glider", "Ravenous Necrotitan", "Sinew Dancer",
	"Vivisection Evangelist", "Zealot's Conviction"}

// poisonhighPolicy casts or activates whenever it can (so the games reach
// the carriers and poison rises), else random; it counts the carrier plays
// the gorge side poses.
func poisonhighPolicy(seed uint64, casts, acts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					if k == ActCast {
						casts[name]++
					} else {
						acts[name]++
					}
				}
			}
		}
		return a
	})
}

// runPoisonhigh plays the constructed matchups and returns the games, the
// divergences and the carrier plays. mutate flips the reader to the
// controller's own poison count (the mutation must diverge).
func runPoisonhigh(t *testing.T, seeds int, mutate bool) (games, divs int, casts, acts map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("poisonhigh")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPoisonHighSelf
	dirac.DebugPoisonHighSelf = mutate
	defer func() { dirac.DebugPoisonHighSelf = old }()
	casts, acts = map[string]int{}, map[string]int{}
	for mi, md := range poisonhighDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"poisonhigh-a", "poisonhigh-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := poisonhighPolicy(seed, casts, acts)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, casts, acts
}

func TestPoisonhighLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, casts, acts := runPoisonhigh(t, 48, false)
	t.Logf("%d games, %d divergences, carrier casts %v, activations %v", games, divs, casts, acts)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, n := range poisonhighCarriers {
		if casts[n] == 0 && acts[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
	for _, n := range []string{"Chittering Skitterling", "Sinew Dancer"} {
		if acts[n] == 0 {
			t.Errorf("carrier %s never activated", n)
		}
	}
}

// TestPoisonhighMutation flips the reader to the controller's own poison
// count: the opponents' highest is what the cards read, so the games must
// diverge. A green mutation (no divergence) means the lockstep never reaches
// a Corrupted decision.
func TestPoisonhighMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runPoisonhigh(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation read the controller's own poison in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
