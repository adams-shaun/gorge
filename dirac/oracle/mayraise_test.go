package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A may-play permission's non-mana RaiseCost$: Noctis,
// Prince of Lucis (cast an artifact from the graveyard, paying 3 life), Alien
// Symbiosis (discarding a card) and Wickerfolk Indomitable (2 life and
// sacrificing an artifact or creature), plus Festival of Embers (the BLB card
// the same lift opens: instants and sorceries from the graveyard on your turn,
// paying 1 life). gorge adds the raise to the cast's
// cost and withholds the offer when it cannot be paid. It also withholds a
// LAND a raising permission grants (Darksteel Citadel under Noctis: a land play
// has no cost site for the raise; mayPlayLandIds).

var kmayraiseDecks = [2][]cardset.Entry{
	{ // A (Esper)
		{Name: "Noctis, Prince of Lucis", Count: 4}, {Name: "Ornithopter", Count: 2}, {Name: "Phyrexian Walker", Count: 2},
		{Name: "Pilgrim's Eye", Count: 2}, {Name: "Prophetic Prism", Count: 2}, {Name: "Careful Study", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 1}, {Name: "Pitiless Plunderer", Count: 2},
		{Name: "Darksteel Citadel", Count: 4}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 4},
	},
	{ // B (B/R)
		{Name: "Alien Symbiosis", Count: 4}, {Name: "Wickerfolk Indomitable", Count: 4}, {Name: "Faithless Looting", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Festival of Embers", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 8},
	},
}

// kmayraiseStats is what the gorge-side event log says: casts from the
// graveyard, by permission.
type kmayraiseStats struct {
	noctis int // an artifact (not Wickerfolk) cast from the graveyard: Noctis's 3 life
	alien  int // Alien Symbiosis cast from the graveyard: a discard
	wicker int // Wickerfolk Indomitable cast from the graveyard: 2 life and a sacrifice
	fest   int // Shock / Lightning Bolt / Careful Study cast from the graveyard (no flashback): Festival of Embers's 1 life
}

type kmayraiseTally struct{ evAt int }

func (p *kmayraiseTally) scan(ref *GorgeSide, st *kmayraiseStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.PutOnStack && e.From == state.ZGraveyard {
			switch chName(g, e.Obj) {
			case "Ornithopter", "Phyrexian Walker", "Pilgrim's Eye", "Prophetic Prism":
				st.noctis++
			case "Alien Symbiosis":
				st.alien++
			case "Wickerfolk Indomitable":
				st.wicker++
			case "Shock", "Lightning Bolt", "Careful Study":
				st.fest++
			}
		}
	}
	p.evAt = len(evs)
}

// kmayraisePolicy plays at random, preferring a graveyard cast when one is
// offered, and counts what the gorge side logs.
func kmayraisePolicy(seed uint64, st *kmayraiseStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6d61797261697365)) // "mayraise"
	var buf []Canon
	var tl kmayraiseTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		if c.D.Kind == DPriority && c.G != nil && prng.IntN(4) != 0 {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZGraveyard {
						buf = append(buf, a)
					}
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// runMayraise plays the matchup in both seatings and returns the games,
// divergences, stalls and the cast counts. mutate skips the raise in the
// kernel (the mutation must diverge).
func runMayraise(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kmayraiseStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mayraise")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugMayRaiseFree
	dirac.DebugMayRaiseFree = mutate
	defer func() { dirac.DebugMayRaiseFree = old }()
	for swap := 0; swap < 2; swap++ {
		md := kmayraiseDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kmayraiseDecks[1], kmayraiseDecks[0]}
		}
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
				t.Fatalf("swap %d deck %d: %d cards", swap, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmayraise-a", "kmayraise-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kmayraisePolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestMayraiseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 games
	games, divs, stalls, st := runMayraise(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Noctis-granted graveyard casts (3 life)", st.noctis}, {"Alien Symbiosis graveyard casts (discard)", st.alien},
		{"Wickerfolk graveyard casts (2 life, sacrifice)", st.wicker}, {"Festival of Embers graveyard casts (1 life)", st.fest},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestMayraiseMutation skips the raise in the kernel: the games must
// diverge. A green mutation means the lockstep never reaches a cast the
// raise shapes.
func TestMayraiseMutation(t *testing.T) {
	games, divs, _, _ := runMayraise(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation skipped the raise in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
