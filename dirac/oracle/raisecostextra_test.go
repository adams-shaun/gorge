package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A RaiseCost static whose Cost$ is a non-mana cost. Terror of the Peaks
// (S:Mode$ RaiseCost | ValidTarget$ Card.Self | Activator$ Player.Opponent |
// Type$ Spell | Cost$ PayLife<3>): a spell an opponent casts that targets it
// costs 3 life more. gorge composes the life into the cast once the chosen
// target matches (composeRaiseCost), drops a target the caster cannot pay
// for, and pays the life with the cast. The expected answer at every decision
// is gorge's.

var kraiseextraDecks = [2][]cardset.Entry{
	{
		{Name: "Terror of the Peaks", Count: 4}, {Name: "Hill Giant", Count: 8}, {Name: "Shifting Grift", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Terror of the Peaks", Count: 2}, {Name: "Hill Giant", Count: 6}, {Name: "Shifting Grift", Count: 3}, {Name: "Shock", Count: 5},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Mountain", Count: 15}, {Name: "Island", Count: 3},
	},
}

// kraiseextraStats counts, from the gorge event log, the spells cast at a
// Terror of the Peaks.
type kraiseextraStats struct {
	opp  int // cast by a Terror's opponent: 3 life more
	two  int // ...of those, a spell with two targets (Shifting Grift's second creature)
	self int // cast by the Terror's own controller: no raise
}

type kraiseextraTally struct {
	evAt int
	pend []state.ObjID // spells on the stack whose targets are not chosen yet
}

func (p *kraiseextraTally) scan(ref *GorgeSide, st *kraiseextraStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.pend = 0, p.pend[:0]
	}
	for j := p.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.PutOnStack {
			p.pend = append(p.pend, e.Obj)
		}
	}
	p.evAt = len(evs)
	keep := p.pend[:0]
	for _, id := range p.pend {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZStack {
			continue
		}
		if len(o.Targets) == 0 {
			keep = append(keep, id)
			continue
		}
		for _, t := range o.Targets {
			if t.IsPlayer || chName(g, t.Obj) != "Terror of the Peaks" {
				continue
			}
			if to := g.Obj(t.Obj); to != nil && to.Controller != o.Controller {
				st.opp++
				if len(o.Targets) > 1 {
					st.two++
				}
			} else if to != nil {
				st.self++
			}
		}
	}
	p.pend = keep
}

// kraiseextraPolicy plays at random and counts what the gorge side logs.
func kraiseextraPolicy(seed uint64, st *kraiseextraStats) Policy {
	rnd := RandomPolicy(seed)
	var tl kraiseextraTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		return rnd.Pick(c)
	})
}

// runRaiseextra plays the matchup in both seatings. mutate skips the life
// raise in the kernel (the mutation must diverge).
func runRaiseextra(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kraiseextraStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("raisecostextra")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRaiseLifeFree
	dirac.DebugRaiseLifeFree = mutate
	defer func() { dirac.DebugRaiseLifeFree = old }()
	for swap := 0; swap < 2; swap++ {
		md := kraiseextraDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kraiseextraDecks[1], kraiseextraDecks[0]}
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
			m, err := NewMatchup(reg, [2]string{"kraiseextra-a", "kraiseextra-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kraiseextraPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestRaisecostExtraCostLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runRaiseextra(t, 400, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.opp < 3 || st.two < 1 {
		t.Errorf("spells cast at a Terror by its opponent: %d, with two targets: %d (want >= 3 and >= 1)", st.opp, st.two)
	}
}

// TestRaisecostExtraCostMutation skips the life raise in the kernel: the
// games must diverge. A green mutation means the lockstep never reaches a cast
// the raise shapes.
func TestRaisecostExtraCostMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runRaiseextra(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation skipped the raise in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
