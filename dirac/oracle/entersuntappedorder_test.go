package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// "Lands you control enter untapped" against a printed-tapped land:
// The Wandering Minstrel and Spelunking (R:Event$ Moved ... Updated, DB$ Untap
// | ETB$ True) beside Dismal Backwater. gorge poses CR 616.1's order for the
// competing tap and untap on the MoveZone event before the move
// (replacement_updated.go composeUpdatedReplacements); the kernel poses the
// same ask with the land still in its hand, on a land play (Backwater) and on
// Spelunking's "put a land card from your hand onto the battlefield".
// M1: carriers and Backwaters against creatures and removal (two carriers and
// a Backwater is a three-way competition that re-asks, posed with the land on
// the battlefield: gorge emits the move at the first answer); M2: the mirror.

var krepletbDecks = [][2][]cardset.Entry{
	{ // M1: Minstrel, Spelunking and Dismal Backwater against creatures and removal
		{
			{Name: "The Wandering Minstrel", Count: 4}, {Name: "Spelunking", Count: 4}, {Name: "Dismal Backwater", Count: 4},
			{Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 9}, {Name: "Island", Count: 9},
		},
		{
			{Name: "Walking Corpse", Count: 5}, {Name: "Gray Ogre", Count: 5}, {Name: "Hill Giant", Count: 4},
			{Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 7},
		},
	},
	{ // M2: a mirror of carriers, Backwaters and removal for them: the asks go to both seats
		{
			{Name: "The Wandering Minstrel", Count: 4}, {Name: "Spelunking", Count: 4}, {Name: "Dismal Backwater", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "The Wandering Minstrel", Count: 4}, {Name: "Spelunking", Count: 4}, {Name: "Dismal Backwater", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 6},
		},
	},
}

// krepletbStats counts what the gorge-side decisions show.
type krepletbStats struct {
	asks   int // CR 616.1 entry-order asks (DReplacement)
	three  int // ... with three options (the first ask of a three-way competition)
	spelun int // ... posed with an object on the stack (Spelunking's land move; a land play needs an empty stack)
}

// krepletbPolicy plays at random among casting, activating and playing lands
// so the carriers and the tapped lands get played, and counts the asks.
func krepletbPolicy(seed uint64, st *krepletbStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x657462)) // "etb"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DReplacement:
			st.asks++
			if len(c.D.Options) == 3 {
				st.three++
			}
			if c.G != nil {
				if len(c.G.E.G.Stack) > 0 {
					st.spelun++ // a land play needs an empty stack: only Spelunking's resolving trigger moves a land otherwise
				}
			}
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
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

// runRepletb plays the constructed matchups, each in both seatings. mutate
// skips the kernel's ask and taps last (the mutation must diverge).
func runRepletb(t *testing.T, seeds int, mutate bool) (games int, divs, stalls [2]int, st [2]krepletbStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repletb")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugEntryOrderTapLast
	dirac.DebugEntryOrderTapLast = mutate
	defer func() { dirac.DebugEntryOrderTapLast = old }()
	for mi, md0 := range krepletbDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"krepletb-a", "krepletb-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, krepletbPolicy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestEntersUntappedReplOrderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runRepletb(t, 96, false)
	t.Logf("%d games, %v divergences, %v stalls", games, divs, stalls)
	t.Logf("M1 %+v", st[0])
	t.Logf("M2 %+v", st[1])
	if divs != [2]int{} || stalls != [2]int{} {
		t.Fatalf("%v divergences, %v stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"entry-order asks", st[0].asks + st[1].asks}, {"three-way (re-asked) competitions", st[0].three + st[1].three},
		{"asks on Spelunking's land move", st[0].spelun + st[1].spelun},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestEntersUntappedReplOrderMutation skips the ask and taps last: the lockstep must
// diverge where gorge poses the order.
func TestEntersUntappedReplOrderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runRepletb(t, 32, true)
	if divs[0]+divs[1] == 0 {
		t.Error("the mutation skipped the entry-order ask with no divergence")
	}
	t.Logf("mutation diverged %v in %d games", divs, games)
}
