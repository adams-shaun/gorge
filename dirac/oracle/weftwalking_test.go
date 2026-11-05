package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Weftwalking lockstep: its enters trigger sweeps your graveyard and
// hand into your library (graveyard cards first in move order) and
// shuffles, and its static makes the ACTIVE player's first spell each turn
// free, whoever controls Weftwalking. Thought Scour and Doom Blade fill
// the graveyard; Air Elemental is the expensive spell worth casting free.

var kweftDeck = []cardset.Entry{
	{Name: "Weftwalking", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 3},
	{Name: "Thought Scour", Count: 3}, {Name: "Air Elemental", Count: 2},
	{Name: "Plains", Count: 12}, {Name: "Island", Count: 12},
}

type kweftObs struct {
	weftCasts int // Weftwalking cast
	moved     int // enters resolved with cards in both graveyard and hand, then graveyard empty
	freeOpp   int // a free cast offered to the active player while the other seat controls Weftwalking
	freeSelf  int // ... while the active player controls it
}

func kweftCount(g *state.Game, z state.Zone, p state.PlayerID, name string) int {
	n := 0
	for _, id := range g.Zone(z, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

func kweftPolicy(seed uint64, obs *kweftObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var prevWeft [2]int
	var pending [2]bool
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			gm := g.E.G
			for p := state.PlayerID(0); p < 2; p++ {
				w := kweftCount(gm, state.ZBattlefield, p, "Weftwalking")
				gr, h := len(gm.Zone(state.ZGraveyard, p)), len(gm.Zone(state.ZHand, p))
				if pending[p] && gr == 0 {
					obs.moved++
					pending[p] = false
				}
				if w > prevWeft[p] && gr > 0 && h >= 2 {
					pending[p] = true
				}
				prevWeft[p] = w
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || c.G == nil {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := c.G.E.G.Obj(o.Obj)
			free := o.AltCostIndex > 0 || o.Mode == "mayplay"
			weft := ob != nil && ob.Face() != nil && ob.Face().Name == "Weftwalking"
			if free {
				p := state.PlayerID(c.D.Player)
				other := 1 - p
				if p == c.G.E.G.Active {
					if kweftCount(c.G.E.G, state.ZBattlefield, other, "Weftwalking") > 0 {
						obs.freeOpp++
					} else if kweftCount(c.G.E.G, state.ZBattlefield, p, "Weftwalking") > 0 {
						obs.freeSelf++
					}
				}
			}
			if free || weft {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || seed%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		a := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Weftwalking" {
				obs.weftCasts++
			}
		}
		return a
	})
}

func TestWeftwalkingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("weftwalking")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kweftDeck {
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
	tbl, _, _, err := set.Matchup(kweftDeck, kweftDeck)
	if err != nil {
		t.Fatal(err)
	}
	obs := &kweftObs{}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kweft-a", "kweft-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kweftPolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls, Weftwalking casts %d, enters sweeps %d, free casts offered to the active player: other seat's Weftwalking %d, own %d",
		games, divs, stalls, obs.weftCasts, obs.moved, obs.freeOpp, obs.freeSelf)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.weftCasts == 0 || obs.moved == 0 {
		t.Errorf("Weftwalking cast %d times, sweeps with both zones %d, want both > 0", obs.weftCasts, obs.moved)
	}
	if obs.freeOpp == 0 {
		t.Errorf("no free cast offered to the active player under the other seat's Weftwalking")
	}
}
