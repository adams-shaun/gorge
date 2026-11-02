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

// The object-form CantPutCounter static (Blossombind): "Enchanted creature
// can't become untapped and can't have counters put on it". The kernel now
// reads StatCantPutCounter at the one counter-placement funnel, before any
// AddCounter replacement. The lockstep pairs Blossombind with counter makers
// that can target the enchanted creature (Atmospheric Greenhouse, Wingspan
// Mentor, Benevolent Hydra, Ajani, Caller of the Pride) beside removal.
//
// gorge decisions are the authority: a CounterChange event on a creature
// enchanted by Blossombind must never appear (the placement is swallowed),
// while counters elsewhere must still land.

var krepl3CpcDecks = [2][]cardset.Entry{
	{
		{Name: "Blossombind", Count: 4}, {Name: "Atmospheric Greenhouse", Count: 4},
		{Name: "Wingspan Mentor", Count: 4}, {Name: "Benevolent Hydra", Count: 3},
		{Name: "Air Elemental", Count: 4}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
	},
	{
		{Name: "Ajani, Caller of the Pride", Count: 4}, {Name: "Duskfang Mentor", Count: 4},
		{Name: "Serra Angel", Count: 4}, {Name: "Void Beckoner", Count: 3},
		{Name: "Walking Corpse", Count: 6}, {Name: "Pacifism", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8},
	},
}

type krepl3CpcStats struct {
	cast        int // Blossombind picked at priority
	hostTargets int // a counter ask aimed at the Blossombind host
	onHost      int // CounterChange placed on the host (must be 0)
	offHost     int // CounterChange placed on any other permanent
}

type krepl3CpcTally struct{ evAt int }

// krepl3CpcHost reports whether id is enchanted by a Blossombind.
func krepl3CpcHost(g *state.Game, id state.ObjID) bool {
	if id == 0 {
		return false
	}
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, b := range g.Zone(state.ZBattlefield, pl) {
			if o := g.Obj(b); o != nil && o.AttachedTo == id && chName(g, b) == "Blossombind" {
				return true
			}
		}
	}
	return false
}

func (p *krepl3CpcTally) scan(ref *GorgeSide, st *krepl3CpcStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.CounterChange || e.Amount <= 0 || e.Obj == 0 {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if krepl3CpcHost(g, e.Obj) {
			st.onHost++
		} else {
			st.offHost++
		}
	}
	p.evAt = len(evs)
}

// krepl3CpcPolicy casts Blossombind whenever it can, aims a counter ask at
// the enchanted creature, and is krepl3Policy otherwise.
func krepl3CpcPolicy(seed uint64, st *krepl3CpcStats) Policy {
	inner := krepl3Policy(seed, &krepl3Stats{})
	prng := rand.New(rand.NewPCG(seed, 0x63707574)) // "cput"
	var tl krepl3CpcTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		g := c.G.E.G
		tl.scan(c.G, st)
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				opt, ok := c.G.GorgeOption(a)
				if ok && opt.Obj != 0 && krepl3CpcHost(g, opt.Obj) {
					st.hostTargets++
					return a
				}
			}
		}
		if c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if opt, ok := c.G.GorgeOption(a); ok && chName(g, opt.Obj) == "Blossombind" && prng.IntN(8) != 0 {
					st.cast++
					return a
				}
			}
		}
		return inner.Pick(c)
	})
}

// runCantputcounterreplCpc plays the matchup in both seatings, seeds 1..seeds each;
// mutate expects divergences and does not report them.
func runCantputcounterreplCpc(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st krepl3CpcStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("anykindcounters")
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		old := dirac.DebugCtrBlockOff
		dirac.DebugCtrBlockOff = true
		defer func() { dirac.DebugCtrBlockOff = old }()
	}
	for swap := 0; swap < 2; swap++ {
		md := krepl3CpcDecks
		if swap == 1 {
			md = [2][]cardset.Entry{md[1], md[0]}
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
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"krepl3-a", "krepl3-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, krepl3CpcPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestCantPutCounterReplLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runCantputcounterreplCpc(t, 48, false)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.cast < 2 {
		t.Errorf("Blossombind casts %d (want >= 2): the matchup did not reach the static", st.cast)
	}
	if st.hostTargets < 3 {
		t.Errorf("counter asks aimed at the Blossombind host %d (want >= 3)", st.hostTargets)
	}
	if st.onHost != 0 {
		t.Errorf("counters placed on a Blossombind host %d (want 0)", st.onHost)
	}
	if st.offHost < 3 {
		t.Errorf("counters placed off the Blossombind host %d (want >= 3): the prohibition blocked everything", st.offHost)
	}
}

// TestCantPutCounterReplMutation turns the kernel's prohibition off: a
// counter the gorge side swallows is placed by the kernel twin, so the
// lockstep must diverge.
func TestCantPutCounterReplMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runCantputcounterreplCpc(t, 8, true)
	if divs == 0 {
		t.Errorf("mutation disabled the CantPutCounter static with no divergence in %d games", games)
	}
	t.Logf("mutation diverged %d of %d games", divs, games)
}
