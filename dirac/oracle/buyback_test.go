package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Buyback lockstep (CR 702.27a): Searing Touch, Capsize, Evincar's
// Justice, Elvish Fury, Mind Games (Buyback {N}) and Forbid (Buyback:
// discard two cards). gorge offers a "buyback" cast beside the plain one (the
// base cost plus the buyback cost) and puts the RESOLVED paid spell into its
// owner's hand; a countered or fizzled one goes to the graveyard. The kernel's
// kicked cast is that option (kw_buyback.go).

var kbuybackDecks = [][2][]cardset.Entry{{
	// A: the burn and bounce, with Forbid against B's spells
	{
		{Name: "Searing Touch", Count: 4}, {Name: "Capsize", Count: 4}, {Name: "Evincar's Justice", Count: 3},
		{Name: "Forbid", Count: 3}, {Name: "Mind Games", Count: 2}, {Name: "Elvish Fury", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 3},
	},
	// B: pump and counter spells over creatures to pump, burn and bounce
	{
		{Name: "Forbid", Count: 4}, {Name: "Capsize", Count: 3}, {Name: "Searing Touch", Count: 3},
		{Name: "Elvish Fury", Count: 4}, {Name: "Evincar's Justice", Count: 2}, {Name: "Mind Games", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Elite Vanguard", Count: 2}, {Name: "Llanowar Elves", Count: 1},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 3},
	},
}}

var kbuybackCards = map[string]bool{
	"Searing Touch": true, "Capsize": true, "Evincar's Justice": true, "Elvish Fury": true, "Forbid": true, "Mind Games": true,
}

// kbuybackPaid: the cast mode of a buyback cast. An offered option names it
// "buyback"; a cast lowered from gorge's potential play carries the cast-mode
// nibble castModeOther, which CastModeName spells "offspring" (no Offspring
// card is in these decks).
func kbuybackPaid(mode string) bool { return mode == "buyback" || mode == "offspring" }

// kbuybackTally counts, per card, the buyback and plain casts the policy
// chose and the plain casts taken while no buyback cast of that spell was on
// offer.
type kbuybackTally struct {
	paid, plain, noOffer map[string]int
}

func newKbuybackTally() *kbuybackTally {
	return &kbuybackTally{map[string]int{}, map[string]int{}, map[string]int{}}
}

// kbuybackPolicy steers the games to the Buyback cards: an offered buyback
// cast is taken two times in three, else a random cast, else random.
func kbuybackPolicy(seed uint64, tl *kbuybackTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x62757962))
	var paid, casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		paid, casts = paid[:0], casts[:0]
		paidObj := map[uint32]bool{}
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || c.G == nil {
				continue
			}
			casts = append(casts, a)
			if o, ok := c.G.GorgeOption(a); ok && kbuybackPaid(o.Mode) {
				paid = append(paid, a)
				paidObj[uint32(o.Obj)] = true
			}
		}
		var a Canon
		switch {
		case seed%4 == 0 || len(casts) == 0:
			a = rnd.Pick(c)
		case len(paid) > 0 && r.IntN(3) != 0:
			a = pick(c, paid)
		default:
			a = pick(c, casts)
		}
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Kind != "cast" {
			return a
		}
		name := c.G.E.G.Obj(o.Obj).Face().Name
		if !kbuybackCards[name] {
			return a
		}
		switch {
		case kbuybackPaid(o.Mode):
			tl.paid[name]++
		case o.Mode == "":
			tl.plain[name]++
			if !paidObj[uint32(o.Obj)] {
				tl.noOffer[name]++
			}
		}
		return a
	})
}

// runBuyback plays the matchups for seeds 1..seeds and returns the games, the
// divergences, the kernel capacity stalls and the tally; mutate sets
// DebugBuybackNeverReturns (the mutation must diverge).
func runBuyback(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl *kbuybackTally) {
	t.Helper()
	tl = newKbuybackTally()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("buyback")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugBuybackNeverReturns
	dirac.DebugBuybackNeverReturns = mutate
	defer func() { dirac.DebugBuybackNeverReturns = old }()
	for mi, md := range kbuybackDecks {
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
			m, err := NewMatchup(reg, [2]string{"kbuyback-a", "kbuyback-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kbuybackPolicy(seed, tl), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 24})
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					lo := max(d.At-12, 0)
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
	return games, divs, stalls, tl
}

func TestBuybackLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runBuyback(t, 200, false)
	t.Logf("%d games, %d divergences, %d stalls; buyback %v, plain %v, plain with no buyback offer %v",
		games, divs, stalls, tl.paid, tl.plain, tl.noOffer)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for name := range kbuybackCards {
		if tl.paid[name] < 10 || tl.plain[name] < 10 {
			t.Errorf("%s: thin coverage: buyback %d, plain %d (want 10 each)", name, tl.paid[name], tl.plain[name])
		}
	}
}

// TestBuybackMutation: DebugBuybackNeverReturns sends a paid spell to the
// graveyard, so the lockstep must diverge.
func TestBuybackMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runBuyback(t, 64, true)
	if divs == 0 {
		t.Fatalf("DebugBuybackNeverReturns: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
