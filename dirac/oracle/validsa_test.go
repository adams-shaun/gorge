package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The SpellCast ValidSA$ heads lockstep: Smaug, Wicked
// Worm's Spell.ManaFromTreasure (the cast spent mana a Treasure produced)
// and Danitha, Sword of Hope's "Spell.Equipment,Spell.IsTargeting Valid
// Creature.YouCtrl" (gorge trigmatch ValidSAMatches, cast.go:731).

var kvalidsaDecks = [][2][]cardset.Entry{{
	// A: Danitha, Equipment, and spells that target your creature
	{
		{Name: "Danitha, Sword of Hope", Count: 4}, {Name: "Quick-Draw Katana", Count: 3}, {Name: "Swiftfoot Boots", Count: 2},
		{Name: "Short Bow", Count: 2}, {Name: "Giant Growth", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 11},
	},
	// B: Smaug, Treasure makers, and burn
	{
		{Name: "Smaug, Wicked Worm", Count: 4}, {Name: "Rapacious Dragon", Count: 3}, {Name: "Gleaming Barrier", Count: 3},
		{Name: "Brass's Bounty", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 12},
	},
}}

// kvalidsaPolicy steers toward casts: seed%4 == 0 is RandomPolicy, else a
// random cast two times in three, else RandomPolicy.
func kvalidsaPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x76616c73))
	var casts []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		casts = casts[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				casts = append(casts, a)
			}
		}
		a := rnd.Pick(c)
		if !(seed%4 == 0 || len(casts) == 0 || r.IntN(3) == 0) {
			d := *c.D
			d.Options = casts
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return a
	})
}

// runValidsa plays the matchups for seeds 1..seeds and returns the games,
// the divergences and the kernel capacity stalls.
func runValidsa(t *testing.T, seeds int, mutate bool) (games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("validsa")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range kvalidsaDecks {
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
			m, err := NewMatchup(reg, [2]string{"kvalidsa-a", "kvalidsa-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kvalidsaPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return games, divs, stalls
}

func TestValidsaLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls := runValidsa(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
}

// TestValidsaMutation: mode 1 (the Treasure gate always holds) and mode 3
// (the target arm never matches) must diverge; mode 2 (the gate never holds)
// is logged only, a Treasure-paid cast with Smaug out being rare.
func TestValidsaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	defer dirac.SetDebugValidSAMutate(0)
	for _, m := range []uint8{1, 2, 3} {
		dirac.SetDebugValidSAMutate(m)
		games, divs, _ := runValidsa(t, 128, true)
		t.Logf("mode %d: diverged in %d of %d games", m, divs, games)
		if m != 2 && divs == 0 {
			t.Errorf("mode %d: no divergence in %d games", m, games)
		}
	}
}
