package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: AtEOT$ on Animate (Coalstoke Gearhulk:
// "exile that creature" at the next end step, one remembered object) and on
// a ChangeZoneAll sweep onto the battlefield (Push the Limit: sacrifice
// every returned Mount and Vehicle at the next end step, one promise per
// entrant, kop.EntryAtEOT). Two mirror decks of black-red cheap creatures,
// self-mill, Vehicles and the two carriers.

var kateotDeck = []cardset.Entry{
	{Name: "Coalstoke Gearhulk", Count: 3}, {Name: "Push the Limit", Count: 4},
	{Name: "Daggerfang Duo", Count: 4}, {Name: "Deathcap Marionette", Count: 4},
	{Name: "Gilded Ghoda", Count: 1}, {Name: "Burner Rocket", Count: 2},
	{Name: "Gastal Thrillroller", Count: 2}, {Name: "Clamorous Ironclad", Count: 1},
	{Name: "Carrion Cruiser", Count: 1},
	{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
}

// kateotAnimDeck carries the Animate shapes whose subject is one object:
// Apprentice Necromancer (an activated ability whose source is sacrificed as
// a cost, Remembered; a card reanimated twice in a turn), Kami of Industry
// (Defined$ Targeted), Chimeric Coils (Self, X paid) and Dawn of the Dead
// (an optional upkeep trigger, Exile).
var kateotAnimDeck = []cardset.Entry{
	{Name: "Apprentice Necromancer", Count: 4}, {Name: "Kami of Industry", Count: 3},
	{Name: "Chimeric Coils", Count: 4}, {Name: "Dawn of the Dead", Count: 2},
	{Name: "Daggerfang Duo", Count: 3}, {Name: "Deathcap Marionette", Count: 3},
	{Name: "Burner Rocket", Count: 2}, {Name: "Gastal Thrillroller", Count: 2},
	{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 7},
}

// kateotScan counts, from gorge's event log, the next-end-step promises
// that fired: a DelayedRegister body __kwWarpExile (Coalstoke) or
// __kwEncoreSacrifice (Push the Limit) whose object then left the battlefield
// during an end step.
func kateotScan(g *GorgeSide, names map[string]int) (exiled, sacked int) {
	if g == nil {
		return
	}
	reg := map[state.ObjID]string{}
	step := state.Step(0)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.DelayedRegister:
			if ev.Counter == "__kwWarpExile" || ev.Counter == "__kwEncoreSacrifice" {
				reg[ev.Obj] = ev.Counter
			}
		case events.MoveZone:
			if ev.From != state.ZBattlefield {
				continue
			}
			body, ok := reg[ev.Obj]
			if !ok {
				continue
			}
			delete(reg, ev.Obj)
			if step != state.StepEnd {
				continue
			}
			switch {
			case body == "__kwWarpExile" && ev.To == state.ZExile:
				exiled++
			case body == "__kwEncoreSacrifice" && ev.To == state.ZGraveyard:
				sacked++
				if o := g.E.G.Obj(ev.Obj); o != nil && names != nil {
					names[o.Face().Name]++
				}
			}
		}
	}
	return
}

func kateotRun(t *testing.T, deck []cardset.Entry, names map[string]int, seeds uint64, mutant bool) (games, exiled, sweeps, sacked int, diverged bool, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ateotmovers")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range deck {
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
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kateot-a", "kateot-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := RandomPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		e, s := kateotScan(gs, names)
		exiled += e
		sacked += s
		if s > 0 {
			sweeps++
		}
		if d := res.Div; d != nil {
			diverged = true
			if mutant {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

const kateotSeeds = 256

func TestAteotmoversLockstep(t *testing.T) {
	t.Parallel()
	games, exiled, sweeps, sacked, _, stalls := kateotRun(t, kateotDeck, nil, kateotSeeds, false)
	t.Logf("%d games: %d Coalstoke reanimations exiled at the end step; %d games with a Push the Limit sweep, %d entrants sacrificed at the end step; kernel stalls %v",
		games, exiled, sweeps, sacked, stalls)
	if exiled < 10 {
		t.Errorf("Coalstoke exiled %d reanimations at the end step, want at least 10", exiled)
	}
	if sweeps < 5 {
		t.Errorf("Push the Limit sacrificed at the end step in %d games, want at least 5", sweeps)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestAteotmoversAnimLockstep: the one-object Animate carriers beside the two
// above, in a deck that reanimates and sacrifices on both seats.
func TestAteotmoversAnimLockstep(t *testing.T) {
	t.Parallel()
	names := map[string]int{}
	games, exiled, sweeps, sacked, _, stalls := kateotRun(t, kateotAnimDeck, names, kateotSeeds, false)
	t.Logf("%d games: %d reanimations exiled at the end step (Dawn of the Dead); %d games with an end-step sacrifice, %d objects sacrificed at the end step %v; kernel stalls %v",
		games, exiled, sweeps, sacked, names, stalls)
	// Kami of Industry returns only artifacts (the Vehicles), Apprentice
	// Necromancer only creatures, Chimeric Coils sacrifices itself.
	if exiled < 5 || names["Chimeric Coils"] < 5 || names["Burner Rocket"]+names["Gastal Thrillroller"] < 5 ||
		names["Daggerfang Duo"]+names["Deathcap Marionette"] < 5 {
		t.Errorf("a carrier is under-reached: exiled %d, sacrificed %v", exiled, names)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestAteotmoversMutation: the sweep's per-entrant registration skipped,
// the lockstep must diverge (Push the Limit's entrants stay on the board
// while gorge sacrifices them at the end step).
func TestAteotmoversMutation(t *testing.T) {
	dirac.DebugAteotMoversSkip = true
	defer func() { dirac.DebugAteotMoversSkip = false }()
	if _, _, _, _, diverged, _ := kateotRun(t, kateotDeck, nil, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with the per-entrant registration skipped")
	}
}
