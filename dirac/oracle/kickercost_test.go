package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The non-mana kicker lockstep (CR 702.33a): Vayne's
// Treachery and Stomped by the Foot (Kicker Sac<1/Creature;Artifact>) and
// Chocobo Kick (Kicker Return<1/Land>). gorge's kickerCost is a full Cost
// the kicked cast adds to its own; the kicked mode is offered only when the
// sacrifice or return has a candidate. The kernel's kicked cast pays
// CardDef.KickerCost through the cost stages (kw_kicker.go).

var kkickercostDecks = [][2][]cardset.Entry{{
	// A: the sacrifice kickers, a death trigger (a sacrifice is visible),
	// fodder creatures and an artifact
	{
		{Name: "Vayne's Treachery", Count: 3}, {Name: "Stomped by the Foot", Count: 3}, {Name: "Vengeful Bloodwitch", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Ornithopter", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Plains", Count: 6},
	},
	// B: the return kicker, a landfall trigger (a returned land is replayed),
	// creatures to pump and to burn, and mana to pay for it
	{
		{Name: "Chocobo Kick", Count: 3}, {Name: "Tatyova, Benthic Druid", Count: 3}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Ornithopter", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	},
}, {
	// C: Chocobo Kick against the sacrifice kickers, so its targets carry them
	{
		{Name: "Chocobo Kick", Count: 4}, {Name: "Tatyova, Benthic Druid", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Savannah Lions", Count: 2},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Vayne's Treachery", Count: 4}, {Name: "Stomped by the Foot", Count: 4}, {Name: "Vengeful Bloodwitch", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Ornithopter", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Plains", Count: 6},
	},
}}

var kkickercostCards = map[string]bool{"Vayne's Treachery": true, "Stomped by the Foot": true, "Chocobo Kick": true}

// kkickercostTally counts, per card, the kicked and plain casts the policy
// chose and the plain casts taken while no kicked cast of that spell was on
// offer (no sacrifice or return candidate).
type kkickercostTally struct {
	kicked, plain, noOffer map[string]int
}

func newKkickercostTally() *kkickercostTally {
	return &kkickercostTally{map[string]int{}, map[string]int{}, map[string]int{}}
}

// kkickercostPolicy steers the games to the kicker cards: an offered kicked
// cast is taken two times in three, else a random cast, else random.
func kkickercostPolicy(seed uint64, tl *kkickercostTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6b69636b))
	var kicked, casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		kicked, casts = kicked[:0], casts[:0]
		kickedObj := map[uint32]bool{}
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || c.G == nil {
				continue
			}
			casts = append(casts, a)
			if o, ok := c.G.GorgeOption(a); ok && o.Mode == "kicked" {
				kicked = append(kicked, a)
				kickedObj[uint32(o.Obj)] = true
			}
		}
		var a Canon
		switch {
		case seed%4 == 0 || len(casts) == 0:
			a = rnd.Pick(c)
		case len(kicked) > 0 && r.IntN(3) != 0:
			a = pick(c, kicked)
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
		if !kkickercostCards[name] {
			return a
		}
		switch o.Mode {
		case "kicked":
			tl.kicked[name]++
		case "":
			tl.plain[name]++
			if !kickedObj[uint32(o.Obj)] {
				tl.noOffer[name]++
			}
		}
		return a
	})
}

// runKickercost plays the matchups for seeds 1..seeds and returns the
// games, the divergences, the kernel capacity stalls and the tally; mutate
// sets DebugKickerCostFree (the mutation must diverge).
func runKickercost(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl *kkickercostTally) {
	t.Helper()
	tl = newKkickercostTally()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("kickercost")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugKickerCostFree
	dirac.DebugKickerCostFree = mutate
	defer func() { dirac.DebugKickerCostFree = old }()
	for mi, md := range kkickercostDecks {
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
			m, err := NewMatchup(reg, [2]string{"kkickercost-a", "kkickercost-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kkickercostPolicy(seed, tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestNonManaKickerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runKickercost(t, 200, false)
	t.Logf("%d games, %d divergences, %d stalls; kicked %v, plain %v, plain with no kicked offer %v",
		games, divs, stalls, tl.kicked, tl.plain, tl.noOffer)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	noOffer := 0
	for name := range kkickercostCards {
		if tl.kicked[name] < 10 || tl.plain[name] < 10 {
			t.Errorf("%s: thin coverage: kicked %d, plain %d (want 10 each)", name, tl.kicked[name], tl.plain[name])
		}
		noOffer += tl.noOffer[name]
	}
	if noOffer < 3 {
		t.Errorf("only %d plain casts with no kicked offer (want 3)", noOffer)
	}
}

// TestNonManaKickerMutation: DebugKickerCostFree pays no kicker cost, so the
// kicked Vayne's Treachery keeps its creature and the lockstep must diverge.
func TestNonManaKickerMutation(t *testing.T) {
	games, divs, _, _ := runKickercost(t, 128, true)
	if divs == 0 {
		t.Fatalf("DebugKickerCostFree: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
