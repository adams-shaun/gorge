package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Officious Interrogation: "This spell costs {W}{U} more to cast for each
// target beyond the first" (S:Mode$ RaiseCost | Cost$ W U | Relative$ True),
// targets any number of players. The kernel prices the second player target
// into the cast; the expected answer at every decision is gorge's.

var kraiserelDecks = [2][]cardset.Entry{
	{
		{Name: "Officious Interrogation", Count: 4}, {Name: "Savannah Lions", Count: 12}, {Name: "Raging Goblin", Count: 8},
		{Name: "Island", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Savannah Lions", Count: 12}, {Name: "Raging Goblin", Count: 8}, {Name: "Shock", Count: 8},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 6},
	},
}

// kraiserelStats counts, from the gorge event log, the Officious casts by
// their number of targets.
type kraiserelStats struct{ one, two int }

type kraiserelTally struct {
	evAt int
	pend []state.ObjID // spells on the stack whose targets are not chosen yet
}

func (p *kraiserelTally) scan(ref *GorgeSide, st *kraiserelStats) {
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
		if chName(g, id) == "Officious Interrogation" {
			if len(o.Targets) == 1 {
				st.one++
			} else {
				st.two++
			}
		}
	}
	p.pend = keep
}

func kraiserelPolicy(seed uint64, st *kraiserelStats) Policy {
	rnd := RandomPolicy(seed)
	var tl kraiserelTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		return rnd.Pick(c)
	})
}

func runRaiserel(t *testing.T, lo, hi int) (games, divs, stalls int, st kraiserelStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("raisecostrelative")
	if err != nil {
		t.Fatal(err)
	}
	_ = dirac.Config{}
	for swap := 0; swap < 2; swap++ {
		md := kraiserelDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kraiserelDecks[1], kraiserelDecks[0]}
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
		for seed := uint64(lo); seed <= uint64(hi); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kraiserel-a", "kraiserel-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kraiserelPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestRaiseCostPerTargetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runRaiserel(t, 1, 100)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.one < 1 || st.two < 1 {
		t.Errorf("Officious casts with one target: %d, with two: %d (want >= 1 each)", st.one, st.two)
	}
}
