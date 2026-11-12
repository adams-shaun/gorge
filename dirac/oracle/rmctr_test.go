package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: constructed matchups over the generated
// rmctr pool. Its carriers are RemoveCounter Choices$ elections (OpSupport's
// remove form):
//   - Dyadrine, Synthesis Amalgam: remove a +1/+1 counter from each of two
//     creatures you control (an exact two), then draw and make a Robot if any
//     counter was removed. gorge asks one counter_pick (Min = Max = 2) when
//     three or more creatures carry a +1/+1 counter and takes every eligible
//     creature, unasked, when two or fewer do.
//   - Garnet, Princess of Alexandria: remove a lore counter from each of any
//     number of Sagas (Min 0, Max the Sagas you control; a Saga with no lore
//     counter is a legal pick that removes nothing), then Garnet gets that
//     many +1/+1 counters.
// Asks are counted from gorge's pending counter_pick decisions (one per
// gorge ask, the kernel's sequential picks being its own framing), forced
// takes from the Dyadrine resolutions that removed a counter beyond the asks.

var krmctrDecks = [2][]cardset.Entry{
	{
		{Name: "Dyadrine, Synthesis Amalgam", Count: 3}, {Name: "Garnet, Princess of Alexandria", Count: 3},
		{Name: "The Birth of Meletis", Count: 3}, {Name: "History of Benalia", Count: 2},
		{Name: "Luminarch Aspirant", Count: 3}, {Name: "Ajani's Pridemate", Count: 3},
		{Name: "Hopeful Initiate", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Swords to Plowshares", Count: 1},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Dyadrine, Synthesis Amalgam", Count: 2}, {Name: "Garnet, Princess of Alexandria", Count: 2},
		{Name: "The Birth of Meletis", Count: 2}, {Name: "History of Benalia", Count: 2},
		{Name: "Luminarch Aspirant", Count: 3}, {Name: "Ajani's Pridemate", Count: 3},
		{Name: "Hopeful Initiate", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Savannah Lions", Count: 2}, {Name: "Swords to Plowshares", Count: 2},
		{Name: "Giant Growth", Count: 1}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
	},
}

const (
	krmctrDyadAsk    = "dyadrine: exact-2 ask"
	krmctrDyadForced = "dyadrine: forced take (<= 2 eligible)"
	krmctrGarnetAsk  = "garnet: ask"
	krmctrGarnetMany = "garnet: >= 2 Sagas in one answer"
	krmctrGarnetZero = "garnet: Done at zero"
)

var krmctrMin = []struct {
	branch string
	min    int
}{
	{krmctrDyadAsk, 10}, {krmctrDyadForced, 5}, {krmctrGarnetAsk, 10}, {krmctrGarnetMany, 2}, {krmctrGarnetZero, 5},
}

func krmctrSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rmctr")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(krmctrDecks[0], krmctrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl
}

// krmctrTracker follows gorge's pending counter_pick decisions through one
// game: the ask's source, and the picks its answer holds.
type krmctrTracker struct {
	last  *decision.Decision
	src   string
	picks int
	n     map[string]int
}

func (k *krmctrTracker) flush() {
	if k.last == nil || k.last.ResumeKind != "counter_pick" || k.src != "Garnet, Princess of Alexandria" {
		return
	}
	switch {
	case k.picks >= 2:
		k.n[krmctrGarnetMany]++
	case k.picks == 0:
		k.n[krmctrGarnetZero]++
	}
}

// pick observes the answer a to the current gorge decision.
func (k *krmctrTracker) pick(c *PickCtx, a Canon) {
	d := c.G.Pending()
	if d != k.last {
		k.flush()
		k.last, k.picks, k.src = d, 0, ""
		if d == nil || d.ResumeKind != "counter_pick" {
			return
		}
		k.src = kremName(c.G.E.G, d.Source)
		switch {
		case k.src == "Dyadrine, Synthesis Amalgam" && d.Min == 2 && d.Max == 2:
			k.n[krmctrDyadAsk]++
		case k.src == "Garnet, Princess of Alexandria":
			k.n[krmctrGarnetAsk]++
		}
	}
	if d != nil && d.ResumeKind == "counter_pick" && a.Kind() >= ActChoose {
		k.picks++
	}
}

// krmctrScan adds one game's Dyadrine resolutions that removed a counter.
func krmctrScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	for i, ev := range evs {
		if ev.Kind != events.Resolve || kremName(gm, ev.Obj) != "Dyadrine, Synthesis Amalgam" {
			continue
		}
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind == events.CounterChange && evs[j].Amount < 0 {
				n["dyadrine: resolutions removing"]++
				break
			}
		}
	}
}

// krmctrPolicy plays the carriers: it casts, activates, plays lands and
// attacks whenever it can, else random.
func krmctrPolicy(seed uint64, tr *krmctrTracker) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand, ActAttack:
				buf = append(buf, a)
			}
		}
		a := Canon(0)
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G != nil {
			tr.pick(c, a)
		}
		return a
	})
}

// krmctrRun plays seeds 1..seeds through tbl, counting branches into n.
func krmctrRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, n map[string]int, logDiv bool) (games int, div bool, stalls int, seed0 uint64) {
	t.Helper()
	decks := kcastDeck(t, reg, krmctrDecks)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krmctr-a", "krmctr-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		tr := &krmctrTracker{n: n}
		inner := krmctrPolicy(seed, tr)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		tr.flush()
		krmctrScan(gs, n)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div, seed0 = true, seed
			if !logDiv {
				t.Logf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			kcastDump(t, gs)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestRmctrLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, tbl := krmctrSetup(t)
	n := map[string]int{}
	games, div, stalls, _ := krmctrRun(t, reg, set, tbl, 256, n, true)
	n[krmctrDyadForced] = n["dyadrine: resolutions removing"] - n[krmctrDyadAsk]
	t.Logf("%d games, diverged %v, kernel stalls %d", games, div, stalls)
	for _, b := range []string{krmctrDyadAsk, krmctrDyadForced, krmctrGarnetAsk, krmctrGarnetMany, krmctrGarnetZero} {
		t.Logf("branch %-42s %d", b, n[b])
	}
	if div {
		return
	}
	for _, m := range krmctrMin {
		if n[m.branch] < m.min {
			t.Errorf("branch %q: %d, want at least %d", m.branch, n[m.branch], m.min)
		}
	}
}

// TestRmctrMutation: with dirac.DebugRmctrNoRemember (the removed counters
// are not remembered) the lockstep must diverge within 64 seeds: Dyadrine
// stops drawing and Garnet's X reads 0.
func TestRmctrMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, tbl := krmctrSetup(t)
	dirac.DebugRmctrNoRemember = true
	defer func() { dirac.DebugRmctrNoRemember = false }()
	n := map[string]int{}
	_, div, _, seed := krmctrRun(t, reg, set, tbl, 64, n, false)
	if !div {
		t.Fatalf("DebugRmctrNoRemember: no divergence in 64 seeds")
	}
	t.Logf("diverged at seed %d", seed)
}
