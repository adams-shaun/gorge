package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The BecomesSaddled lockstep: Stubborn Burrowfiend's
// "whenever this becomes saddled for the first time each turn, mill two
// cards, then it gets +X/+X" (gorge trigmatch/saddled.go: the Saddled grant
// each Saddle resolution emits, latched by FirstTimeSaddled$). The policy
// saddles whenever it can, including a Mount already saddled this turn, so
// the second-grant arm (no trigger) is exercised. The expected answer at
// every decision is gorge's.
var kotjSaddledDecks = [2][]cardset.Entry{
	{
		{Name: "Stubborn Burrowfiend", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

const kotjMount = "Stubborn Burrowfiend"

// kotjSaddledTally counts gorge's Saddled grants to a Burrowfiend: the first
// of a turn (the trigger goes on the stack) and each later one (it does not).
type kotjSaddledTally struct {
	evAt         int
	turnSeen     map[state.ObjID]bool
	first, later int
}

func (p *kotjSaddledTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.TurnChange:
			clear(p.turnSeen)
		case events.IsAlterAttribute(*e, "Saddled") && e.Amount >= 1:
			if o := g.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != kotjMount {
				continue
			}
			if p.turnSeen[e.Obj] {
				p.later++
			} else {
				p.first++
				p.turnSeen[e.Obj] = true
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// kotjSaddledPolicy saddles a Burrowfiend whenever the activation is offered,
// else casts or plays a land first, else picks at random.
func kotjSaddledPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActActivate || c.G == nil {
				continue
			}
			if op, ok := c.G.GorgeOption(a); ok && op.Obj != 0 {
				if ob := c.G.E.G.Obj(op.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == kotjMount {
					return a
				}
			}
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestOtjSaddledLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("otjsaddled")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kotjSaddledDecks {
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
	tbl, _, _, err := set.Matchup(kotjSaddledDecks[0], kotjSaddledDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	first, later, div, stalls := 0, 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"saddled-a", "saddled-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tl := &kotjSaddledTally{turnSeen: map[state.ObjID]bool{}}
		res := Lockstep(m, seed, k, kotjSaddledPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tl.check})
		CloseSide(k)
		first += tl.first
		later += tl.later
		if res.Div != nil {
			div++
			t.Errorf("seed %d: diverged at %d: %v", seed, res.Div.At, res.Div)
		}
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
	}
	t.Logf("first-of-turn Burrowfiend saddles %d, later saddles %d, divergences %d, kernel stalls %d", first, later, div, stalls)
	if first < 10 {
		t.Errorf("only %d first-of-turn saddles, want >= 10", first)
	}
	if later < 10 {
		t.Errorf("only %d later saddles in a turn, want >= 10", later)
	}
}
