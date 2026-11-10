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

// The CantBeCast origin lockstep: Avatar's Wrath airbends every other
// creature and leaves "until your next turn, your opponents can't cast
// spells from anywhere other than their hands" (Card.!wasCastFromTheirHand).
// The kernel must withhold the opponent's {2} airbend recast from exile
// while gorge does, keep the hand casts offered, and offer the recast again
// once the caster's next turn begins.

var kwrathDecks = [2][]cardset.Entry{
	{
		{Name: "Avatar's Wrath", Count: 4}, {Name: "Aang, the Last Airbender", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Opt", Count: 4}, {Name: "Stroke of Genius", Count: 2},
		{Name: "Plains", Count: 19},
	},
	kairbendDecks[1],
}

type kwrathTally struct {
	kairbendTally
	active     bool
	by         state.PlayerID
	turn       int32
	wraths     int // resolved Avatar's Wrath
	withheld   int // window frames: the opponent has a live airbent card, {2} floating, timing met, no airbend_cast offered
	leaked     int // window frames with an airbend_cast offered to the wrath's opponent (must stay 0)
	handOffers int // window frames where the wrath's opponent is offered a hand cast
	returned   int // frames after the window where the former opponent is offered an airbend_cast
}

func (w *kwrathTally) check(ref *GorgeSide, k Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	for j := w.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.MoveZone || e.From != state.ZStack || e.To != state.ZExile {
			continue
		}
		if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Avatar's Wrath" {
			w.active, w.by, w.turn = true, o.Owner, g.Turn
			w.wraths++
		}
	}
	d := w.kairbendTally.check(ref, k) // advances evAt
	if d != nil || ref.fr == nil || ref.fr.d == nil {
		return d
	}
	if w.active && g.Active == w.by && g.Turn > w.turn {
		w.active = false
	}
	fr := ref.fr
	p := state.PlayerID(fr.d.Player)
	var airbend, hand bool
	for _, o := range fr.d.Options {
		if o.Kind != "cast" {
			continue
		}
		if o.Mode == "airbend_cast" {
			airbend = true
		} else if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZHand {
			hand = true
		}
	}
	if w.wraths == 0 || p == w.by {
		return nil
	}
	if w.active {
		if airbend {
			w.leaked++
		}
		if hand {
			w.handOffers++
		}
		if !airbend && int(p) < len(g.Players) && g.Players[p].Pool.Total() >= 2 && w.owns(g, p) &&
			g.Step.IsMain() && len(g.Stack) == 0 && g.Active == p {
			w.withheld++
		}
	} else if airbend {
		w.returned++
	}
	return nil
}

func kwrathPolicy(seed uint64, w *kwrathTally) Policy {
	inner := kairbendPolicy(seed, &w.kairbendTally)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.G.fr != nil && c.G.fr.d != nil {
			for _, pe := range c.G.fr.prio {
				if pe.a.Kind() != ActCast || pe.opt >= len(c.G.fr.d.Options) {
					continue
				}
				if strings.Contains(c.G.fr.d.Options[pe.opt].Label, "Avatar's Wrath") {
					return pe.a
				}
			}
		}
		return inner.Pick(c)
	})
}

// runKwrath plays the matchup over seeds 1..seeds; ignored sets the
// DebugRestrProvIgnored mutation.
func runKwrath(t *testing.T, seeds int, ignored bool) (*kwrathTally, int, int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("airbend")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwrathDecks {
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
	tbl, _, _, err := set.Matchup(kwrathDecks[0], kwrathDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRestrProvIgnored
	dirac.DebugRestrProvIgnored = ignored
	defer func() { dirac.DebugRestrProvIgnored = old }()
	w := &kwrathTally{kairbendTally: kairbendTally{bender: map[state.ObjID]state.PlayerID{}, tokens: map[state.ObjID]bool{}}}
	games, divs := 0, 0
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwrath-a", "kwrath-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		w.evAt, w.pending, w.active = 0, w.pending[:0], false
		res := Lockstep(m, seed, k, kwrathPolicy(seed, w), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: w.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && !ignored {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if !ignored {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return w, games, divs
}

func TestCantBeCastOriginLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	w, games, divs := runKwrath(t, 256, false)
	t.Logf("%d games, %d divergences: %d wraths, %d window frames withheld the airbend recast, %d hand-cast offers in the window, %d leaked, %d recast offers after the window",
		games, divs, w.wraths, w.withheld, w.handOffers, w.leaked, w.returned)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if w.leaked != 0 {
		t.Errorf("%d window frames offered an airbend recast to the wrath's opponent", w.leaked)
	}
	if w.wraths < 20 {
		t.Errorf("%d Avatar's Wrath resolutions, want at least 20", w.wraths)
	}
	if w.withheld < 5 {
		t.Errorf("%d withheld window frames, want at least 5", w.withheld)
	}
	if w.handOffers < 5 {
		t.Errorf("%d hand-cast offers in the window, want at least 5", w.handOffers)
	}
	if w.returned < 5 {
		t.Errorf("%d recast offers after the window, want at least 5", w.returned)
	}
}

// TestCantBeCastOriginMutation reads every origin clause as holding
// (DebugRestrProvIgnored): the kernel then withholds the opponent's hand
// casts in the window too, so the games must diverge from gorge.
func TestCantBeCastOriginMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	_, games, divs := runKwrath(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation left the origin read off in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
