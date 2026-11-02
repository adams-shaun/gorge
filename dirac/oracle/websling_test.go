package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Web-slinging lockstep (CR 702.188): constructed G/W decks over the
// generated websling pool, so every hand "web-slinging" cast gorge offers,
// and its return-a-tapped-creature pick, is answered by the kernel's
// AkWebSling cast and its ChCostReturn stage.

var webSlingDecks = [2][]cardset.Entry{
	{
		{Name: "Spider-Man, Web-Slinger", Count: 4}, {Name: "Spider-UK", Count: 2}, {Name: "Silk, Web Weaver", Count: 1},
		{Name: "Spider-Man India", Count: 1}, {Name: "Spider-Man, Brooklyn Visionary", Count: 2},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Lotus Cobra", Count: 4}, {Name: "Master Decoy", Count: 1},
		{Name: "Mardu Warshrieker", Count: 4}, {Name: "Burning-Tree Emissary", Count: 1},
		{Name: "Savannah Lions", Count: 2}, {Name: "Serra Angel", Count: 1},
		{Name: "Plains", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Spider-Man, Web-Slinger", Count: 4}, {Name: "Spider-UK", Count: 2}, {Name: "Silk, Web Weaver", Count: 1},
		{Name: "Spider-Man India", Count: 1}, {Name: "Spider-Man, Brooklyn Visionary", Count: 2},
		{Name: "Elvish Mystic", Count: 2}, {Name: "Lotus Cobra", Count: 4}, {Name: "Master Decoy", Count: 1},
		{Name: "Mardu Warshrieker", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Serra Angel", Count: 1}, {Name: "Giant Growth", Count: 1},
		{Name: "Plains", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
	},
}

type webSlingStats struct {
	casts, multi, noneTapped int
}

// webSlingProbe wraps a seeded random policy biased to cast, counting the
// web-slinging casts, those with two or more tapped creatures to pick
// from, and the priority windows where the actor holds a carrier and
// controls creatures, none tapped, and no web-slinging cast is offered.
type webSlingProbe struct {
	st   *webSlingStats
	base Policy
	r    *rand.Rand
}

// webSlingCreatures counts p's creatures and its tapped ones in gorge.
func webSlingCreatures(g *state.Game, p state.PlayerID) (all, tapped int) {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().IsCreature() {
			all++
			if o.Tapped {
				tapped++
			}
		}
	}
	return
}

// webSlingInHand reports whether p's hand holds a web-slinging card.
func webSlingInHand(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			for _, kw := range o.Face().Keywords {
				if strings.HasPrefix(kw, "Web-slinging:") {
					return true
				}
			}
		}
	}
	return false
}

// webSlingCobra reports whether p controls a Lotus Cobra (a land played
// floats a mana).
func webSlingCobra(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().Name == "Lotus Cobra" {
			return true
		}
	}
	return false
}

func (p *webSlingProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	if c.D.Kind == DAttackers {
		// attack often: an attacker stays tapped into the second main
		// phase, where the return can take it
		var atk []Canon
		for _, x := range c.D.Options {
			if x.Kind() == ActAttack {
				atk = append(atk, x)
			}
		}
		if len(atk) > 0 && p.r.IntN(4) != 0 {
			a = atk[p.r.IntN(len(atk))]
		}
		return a
	}
	if c.D.Kind != DPriority {
		return a
	}
	var ws, casts, lands []Canon
	for _, x := range c.D.Options {
		switch x.Kind() {
		case ActCast:
			casts = append(casts, x)
		case ActPlayLand:
			lands = append(lands, x)
		}
		if o, ok := c.G.GorgeOption(x); ok && o.Mode == "web-slinging" {
			ws = append(ws, x)
		}
	}
	g := c.G.E.G
	pl := state.PlayerID(c.D.Player)
	all, tapped := webSlingCreatures(g, pl)
	main := g.Active == pl && (g.Step == state.StepMain1 || g.Step == state.StepMain2)
	if main && len(ws) == 0 && all > 0 && tapped == 0 && webSlingInHand(g, pl) {
		p.st.noneTapped++
	}
	switch {
	case len(ws) > 0 && p.r.IntN(8) != 0:
		a = ws[p.r.IntN(len(ws))]
	case len(lands) > 0 && g.Step == state.StepMain1 && webSlingCobra(g, pl) && webSlingInHand(g, pl) && p.r.IntN(4) != 0:
		// hold the land drop for the second main phase, where a Lotus
		// Cobra's mana meets the tapped attackers
		if a.Kind() == ActPlayLand {
			a = Pack(ActPass, RefNone, RefNone, 0)
			for _, x := range c.D.Options {
				if x.Kind() != ActPlayLand {
					a = x
					break
				}
			}
		}
	case len(lands) > 0 && g.Step == state.StepMain2 && p.r.IntN(4) != 0:
		a = lands[p.r.IntN(len(lands))]
	case len(casts) > 0 && p.r.IntN(4) != 0:
		a = casts[p.r.IntN(len(casts))]
	}
	if o, ok := c.G.GorgeOption(a); ok && o.Mode == "web-slinging" {
		p.st.casts++
		if tapped >= 2 {
			p.st.multi++
		}
	}
	return a
}

// webSlingSetup builds the matchup's corpus decks and table.
func webSlingSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("websling")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range webSlingDecks {
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
	tbl, _, _, err := set.Matchup(webSlingDecks[0], webSlingDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// webSlingRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func webSlingRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st webSlingStats) {
	t.Helper()
	reg, set, tbl, decks := webSlingSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"websling-a", "websling-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &webSlingProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x7765626c))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestWebslingLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := webSlingRun(t, 256, true)
	t.Logf("256 games, web-slinging %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.casts, 40, "web-slinging casts"},
		{st.multi, 10, "casts with two or more tapped creatures"},
		{st.noneTapped, 10, "priority windows with only untapped creatures and no web-slinging offer"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestWebslingMutation: an untapped creature paying the return must make
// the lockstep diverge.
func TestWebslingMutation(t *testing.T) {
	dirac.DebugWebSlingUntapped = true
	defer func() { dirac.DebugWebSlingUntapped = false }()
	if diverged, _, _ := webSlingRun(t, 64, false); !diverged {
		t.Fatal("DebugWebSlingUntapped: no divergence in 64 seeds")
	}
}
