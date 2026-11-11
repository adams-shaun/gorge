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

// Fireball: "This spell costs {1} more to cast for each target beyond the
// first. Fireball deals X damage divided evenly, rounded down, among any
// number of targets." (S:Mode$ RaiseCost | Relative$ True with no Cost$;
// DealDamage DivideEvenly$ RoundedDown). The kernel prices every target beyond
// the first into the cast and deals X over the targets still legal as it
// resolves; the expected answer at every decision is gorge's.

var kdivDecks = [2][]cardset.Entry{
	{
		{Name: "Fireball", Count: 4}, {Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 20},
	},
	{
		{Name: "Raging Goblin", Count: 12}, {Name: "Savannah Lions", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}

// kdivStats counts, from the gorge event log, the Fireball casts by their
// number of targets.
type kdivStats struct{ none, one, many int }

type kdivTally struct {
	evAt int
	pend []state.ObjID // spells on the stack whose targets are not chosen yet
}

func (p *kdivTally) scan(ref *GorgeSide, st *kdivStats) {
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
		if chName(g, id) != "Fireball" {
			continue
		}
		switch n := len(o.Targets); {
		case n == 0:
			// a Fireball with no target yet may still choose them: waits (the
			// target ask is answered before the next pick sees the object)
			keep = append(keep, id)
		case n == 1:
			st.one++
		default:
			st.many++
		}
	}
	p.pend = keep
}

func kdivPolicy(seed uint64, st *kdivStats) Policy {
	rnd := RandomPolicy(seed)
	var tl kdivTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		return rnd.Pick(c)
	})
}

// A multi-object target slot holds eight objects (maxXTgt): a random "any
// number of targets" pick that overflows it ends the game loudly with the
// declared capacity stall StallXTgt, which is no divergence.
func runDivideEvenly(t *testing.T, lo, hi int) (games, divs, stalls, capped int, st kdivStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("divideevenly")
	if err != nil {
		t.Fatal(err)
	}
	_ = dirac.Config{}
	for swap := 0; swap < 2; swap++ {
		md := kdivDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kdivDecks[1], kdivDecks[0]}
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
			m, err := NewMatchup(reg, [2]string{"kdiv-a", "kdiv-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kdivPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) && strings.HasSuffix(res.Stall, "xtgt") {
				capped++
			} else if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, capped, st
}

func TestDivideEvenlyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, capped, st := runDivideEvenly(t, 1, 100)
	t.Logf("%d games, %d divergences, %d stalls, %d target-capacity stalls, %+v", games, divs, stalls, capped, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.many < 1 {
		t.Errorf("Fireball casts with several targets: %d (want >= 1)", st.many)
	}
}
