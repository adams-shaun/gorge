package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Daniels, Temporal Agent: the graveyard ability "{2}, Put this card from
// your graveyard on the bottom of your library: You gain 2 life"
// (PutCardToLibFromGrave<1/-1/CARDNAME>, kop.Cost.PutLibSelf) and the
// enters trigger returning a creature card from the graveyard to its
// owner's hand. The matchup fills the graveyard with discard/surveil and
// has Sela, Tal Shiar Paradox exiling graveyard cards from the other seat.
var danielsDecks = [2][]cardset.Entry{
	{
		{Name: "Daniels, Temporal Agent", Count: 4},
		{Name: "Cloistered Telepath", Count: 4},
		{Name: "Cybernetic Specialist", Count: 4},
		{Name: "Pattern Buffer Preservation", Count: 4},
		{Name: "Moopsy", Count: 4},
		{Name: "Mugato", Count: 4},
		{Name: "Swamp", Count: 8},
		{Name: "Forest", Count: 8},
	},
	{
		{Name: "Sela, Tal Shiar Paradox", Count: 4},
		{Name: "Mugato", Count: 4},
		{Name: "Moopsy", Count: 4},
		{Name: "Academy Instructor", Count: 4},
		{Name: "Cloistered Telepath", Count: 4},
		{Name: "Swamp", Count: 10},
		{Name: "Forest", Count: 10},
	},
}

// danielsObs counts the graveyard windows the policy saw from gorge's side.
type danielsObs struct {
	acts     int // a Daniels graveyard activation was chosen
	windows  int // priority with Daniels in its owner's graveyard
	withheld int // such a window offered no Daniels activation
}

func danielsInGrave(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Daniels, Temporal Agent" {
			return true
		}
	}
	return false
}

// danielsPolicy prefers casts and activations (graveyard ones included) and
// records the observations from the gorge side.
func danielsPolicy(seed uint64, obs *danielsObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	isDanielsAct := func(c *PickCtx, a Canon) bool {
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return false
		}
		ob := c.G.E.G.Obj(o.Obj)
		return ob != nil && ob.Zone == state.ZGraveyard && ob.Face() != nil && ob.Face().Name == "Daniels, Temporal Agent"
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			p := state.PlayerID(c.D.Player)
			if danielsInGrave(c.G.E.G, p) {
				obs.windows++
				offered := false
				for _, a := range c.D.Options {
					if a.Kind() == ActActivate && isDanielsAct(c, a) {
						offered = true
					}
				}
				if !offered {
					obs.withheld++
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 || seed%4 == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActActivate && c.G != nil && isDanielsAct(c, a) {
			obs.acts++
		}
		return a
	})
}

// runDaniels plays the constructed matchup; mutate makes the kernel shuffle
// after the self-bottom cost (the mutation must diverge).
func runDaniels(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *danielsObs) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	old := dirac.DebugPutLibShuffle
	dirac.DebugPutLibShuffle = mutate
	defer func() { dirac.DebugPutLibShuffle = old }()
	obs = &danielsObs{}
	md := danielsDecks
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
		m, err := NewMatchup(reg, [2]string{"kdaniels-a", "kdaniels-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, danielsPolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, obs
}

func TestDanielsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, obs := runDaniels(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, Daniels graveyard activations %d, priority windows with Daniels in the graveyard %d (no activation offered: %d)",
		games, divs, stalls, obs.acts, obs.windows, obs.withheld)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.acts < 10 {
		t.Errorf("%d Daniels graveyard activations, want >= 10", obs.acts)
	}
	if obs.withheld < 10 {
		t.Errorf("%d windows with Daniels in the graveyard and no activation offered, want >= 10", obs.withheld)
	}
}

// TestDanielsMutation makes the kernel shuffle the library after the
// self-bottom cost: gorge does not, so the games must diverge.
func TestDanielsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runDaniels(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation did not diverge in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
