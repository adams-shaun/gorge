package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The ChangesZoneAll own-Remembered lockstep: Blood
// Spatter Analysis's "whenever one or more creatures die" body remembers only
// what the body itself remembered (its RememberSacrificed), never the batch
// gorge's closeZoneBatch seeds into Ctx.Remembered/Captured: every plain
// Remembered reader goes through rememberedExcludingCapture
// (engine/effects/count_ref.go). The body mills, adds a BLOODSTAIN counter and
// sacrifices itself at five or more. Day of Judgment is a multi-death batch;
// Pyroclasm and Fiery Cannonade kill through state-based lethal damage, whose
// creature casualties of one pass are one zone batch too (gorge
// destroyLethalDamage).

var kczallremDecks = [][2][]cardset.Entry{{
	// A: Blood Spatter Analysis, cheap creatures to die, burn to kill them
	{
		{Name: "Blood Spatter Analysis", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Gleaming Barrier", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Pyroclasm", Count: 3}, {Name: "Fiery Cannonade", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 12},
	},
	// B: vanilla bodies and Day of Judgment, the multi-death zone batch
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Jolly Gerbils", Count: 4},
		{Name: "Day of Judgment", Count: 4}, {Name: "Giant Growth", Count: 2},
		{Name: "Plains", Count: 14}, {Name: "Forest", Count: 12},
	},
}}

// kczallremTally counts the carriers' casts the policy chose (gorge options).
type kczallremTally struct {
	bsa, doj int
}

// kczallremPolicy steers the games to Blood Spatter Analysis and Day of
// Judgment: a random ActCast two times in three when one is offered, else a
// random option; every fourth seed is pure random.
func kczallremPolicy(seed uint64, tl *kczallremTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x637a616c))
	var casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		casts = casts[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				casts = append(casts, a)
			}
		}
		var a Canon
		switch {
		case seed%4 == 0 || len(casts) == 0:
			a = rnd.Pick(c)
		case r.IntN(3) != 0:
			a = pick(c, casts)
		default:
			a = rnd.Pick(c)
		}
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Kind != "cast" {
			return a
		}
		switch c.G.E.G.Obj(o.Obj).Face().Name {
		case "Blood Spatter Analysis":
			tl.bsa++
		case "Day of Judgment":
			tl.doj++
		}
		return a
	})
}

// runCzallrem plays the matchup for seeds 1..seeds and returns the games,
// the divergences, the kernel capacity stalls and the tally; mutate sets
// DebugCzallNoLatch (the mutation must diverge).
func runCzallrem(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl kczallremTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czallrem")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCzallNoLatch
	dirac.DebugCzallNoLatch = mutate
	defer func() { dirac.DebugCzallNoLatch = old }()
	for mi, md := range kczallremDecks {
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
			m, err := NewMatchup(reg, [2]string{"kczallrem-a", "kczallrem-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kczallremPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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
	return games, divs, stalls, tl
}

func TestCzallremLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runCzallrem(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, Blood Spatter Analysis casts %d, Day of Judgment casts %d",
		games, divs, stalls, tl.bsa, tl.doj)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.bsa < 40 || tl.doj < 20 {
		t.Errorf("thin coverage: Blood Spatter Analysis casts %d (want 40), Day of Judgment casts %d (want 20)", tl.bsa, tl.doj)
	}
}

// TestCzallremMutation: DebugCzallNoLatch queues a Tf2Batch line per member
// instead of latching it per batch, so a Day of Judgment killing two creatures
// fires the batch line twice and the lockstep must diverge.
func TestCzallremMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runCzallrem(t, 96, true)
	if divs == 0 {
		t.Fatalf("DebugCzallNoLatch: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
