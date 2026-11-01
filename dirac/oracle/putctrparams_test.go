package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: PutCounter's Choices$ recipient pick
// (gorge putCounterChooseTitled: Aerith Rescue Mission's stun counter on one
// of the creatures it tapped, Wick, the Whorled Mind's +1/+1 counter on a
// Snail you control) and RememberPut$ (Systems Override: ten charge counters
// on a stolen Spacecraft, removed again at the next end step). An empty pool
// poses nothing, a lone candidate is taken unasked, two or more pose one
// mandatory counter_pick KChoose. P1's Wurmwall Sweeper is the Spacecraft
// Systems Override sometimes takes; the rest of its targets are creatures.

var kputctrparamsDecks = [2][]cardset.Entry{
	{
		{Name: "Aerith Rescue Mission", Count: 4}, {Name: "Wick, the Whorled Mind", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Systems Override", Count: 4}, {Name: "Wurmwall Sweeper", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 14}, {Name: "Forest", Count: 6},
	},
}

// kputctrparamsTally counts, from gorge's decisions and events, the
// counter_pick asks, the Choices$ resolutions that took a lone candidate
// unasked, Aerith's tap mode with no target (no counter), and Systems
// Override on a Spacecraft (the end step's ten-charge removal) and on
// anything else (no counter).
type kputctrparamsTally struct {
	evAt            int
	lastSeq         uint64
	asks, autoTake  int
	empty           int
	scRemove, nonSC int
}

func (y *kputctrparamsTally) reset() { y.evAt, y.lastSeq = 0, 0 }

func (y *kputctrparamsTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < y.evAt {
		y.evAt = 0
	}
	aerith, wick, override := false, false, false
	put, charge10, token := false, false, false
	for j := y.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			o := g.Obj(e.Obj)
			if o == nil {
				break
			}
			if o.Ability != nil {
				wick = wick || chName(g, o.Source) == "Wick, the Whorled Mind"
			} else {
				switch chName(g, e.Obj) {
				case "Aerith Rescue Mission":
					aerith = true
				case "Systems Override":
					override = true
				}
			}
		case events.TokenCreate:
			token = true
		case events.CounterChange:
			switch {
			case strings.EqualFold(e.Counter, "CHARGE") && e.Amount == 10:
				charge10 = true
			case strings.EqualFold(e.Counter, "CHARGE") && e.Amount == -10:
				y.scRemove++
			case e.Amount > 0 && !strings.EqualFold(e.Counter, "CHARGE"):
				put = true
			}
		}
	}
	y.evAt = len(evs)
	if override && !charge10 {
		y.nonSC++
	}
	if d := ref.Pending(); d != nil && d.ResumeKind == "counter_pick" {
		if uint64(d.Seq) != y.lastSeq {
			y.lastSeq = uint64(d.Seq)
			y.asks++
		}
	} else if aerith || wick {
		switch {
		case put:
			y.autoTake++
		case aerith && !token:
			y.empty++
		}
	}
	return nil
}

func kputctrparamsSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("putctrparams")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kputctrparamsRun plays seeds 1..seeds and returns the first divergence's
// seed (0 none).
func kputctrparamsRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, y *kputctrparamsTally, mutated bool) (games int, divSeed uint64, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kputctrparamsDecks {
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
	tbl, _, _, err := set.Matchup(kputctrparamsDecks[0], kputctrparamsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kputctrparams-a", "kputctrparams-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		y.reset()
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			if !mutated {
				lo := max(d.At-12, 0)
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			return games, seed, stalls
		}
	}
	return games, 0, stalls
}

func TestPutctrparamsLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kputctrparamsSetup(t)
	y := &kputctrparamsTally{}
	games, div, stalls := kputctrparamsRun(t, reg, set, 256, y, false)
	t.Logf("%d games, diverged seed %d, stalls %d; %d counter_pick asks, %d lone-candidate auto-takes, %d empty Aerith tap modes, %d Spacecraft charge removals, %d Systems Override on a non-Spacecraft",
		games, div, stalls, y.asks, y.autoTake, y.empty, y.scRemove, y.nonSC)
	if div != 0 || stalls != 0 {
		return
	}
	if y.asks < 10 {
		t.Errorf("%d counter_pick asks, want at least 10", y.asks)
	}
	if y.autoTake < 5 {
		t.Errorf("%d lone-candidate auto-takes, want at least 5", y.autoTake)
	}
	if y.empty < 3 {
		t.Errorf("%d empty Aerith tap modes, want at least 3", y.empty)
	}
	if y.scRemove < 3 {
		t.Errorf("%d Spacecraft charge removals, want at least 3", y.scRemove)
	}
	if y.nonSC < 3 {
		t.Errorf("%d Systems Override on a non-Spacecraft, want at least 3", y.nonSC)
	}
}

// TestPutctrparamsMutation: a kernel that asks a lone Choices$ candidate
// (gorge takes it unasked), or one that never remembers RememberPut$'s
// recipient (no delayed removal, the ten charge counters stay), must
// diverge.
func TestPutctrparamsMutation(t *testing.T) {
	reg, set := kputctrparamsSetup(t)
	for _, c := range []struct {
		name string
		flag *bool
	}{{"always-ask", &dirac.DebugPutCtrPickAlwaysAsk}, {"remput-never", &dirac.DebugRemPutNever}} {
		t.Run(c.name, func(t *testing.T) {
			*c.flag = true
			defer func() { *c.flag = false }()
			_, div, _ := kputctrparamsRun(t, reg, set, 64, &kputctrparamsTally{}, true)
			if div == 0 {
				t.Fatalf("%s: no divergence in 64 seeds", c.name)
			}
			t.Logf("diverged at seed %d", div)
		})
	}
}
