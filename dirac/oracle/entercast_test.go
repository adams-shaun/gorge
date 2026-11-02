package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The entering spell's cast record: Freestrider
// Commando's K:etbCounter replacement reads Count$CastTotalManaSpent (two
// counters when it was not cast or no mana was spent: a plotted free cast, a
// reanimation by Dread Return), and Rocket-Powered Goblin Glider's ETB
// trigger reads wasCastFromYourGraveyard (a mayhem cast from the graveyard
// after Tormenting Voice / Thrill of Possibility discarded it; a hand cast
// has no trigger). Burning-Tree Emissary floats the mana gorge's planner
// needs to offer the mayhem cast (mayhem_test.go).

var kentercastDecks = [2][]cardset.Entry{
	{ // A: G/B
		{Name: "Freestrider Commando", Count: 4}, {Name: "Dread Return", Count: 3}, {Name: "Murder", Count: 4},
		{Name: "Grizzly Bears", Count: 11}, {Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{ // B: R/x
		{Name: "Rocket-Powered Goblin Glider", Count: 4}, {Name: "Tormenting Voice", Count: 3}, {Name: "Thrill of Possibility", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4}, {Name: "Mountain", Count: 17}, {Name: "Plains", Count: 1},
	},
}

type kentercastStats struct {
	freeWith, freeWithout     int // Freestrider entries, by counters, split below
	castWith, castWithout     int // cast with mana
	plotWith, plotWithout     int // plotted, cast free from exile
	reanimWith, reanimWithout int // entered from the graveyard (Dread Return)
	gliderGrave, gliderHand   int // Glider entries cast from the graveyard (the trigger fires) and from a hand (no trigger)
	games                     int
	offers, discarded         int // mayhem offers seen; decisions with a discarded Glider in the graveyard
}

// kentercastProbe tallies each battlefield entry once (by the entering
// event's index in the log) and biases the base policy toward the branches.
type kentercastProbe struct {
	st   *kentercastStats
	base Policy
	r    *rand.Rand
	seen map[int]bool
}

// entry finds object id's latest battlefield entry in the log: its index and
// the zone it came from.
func (p *kentercastProbe) entry(g *GorgeSide, id state.ObjID) (idx int, from state.Zone) {
	idx = -1
	for i, ev := range g.E.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZBattlefield {
			idx, from = i, ev.From
		}
	}
	return
}

func (p *kentercastProbe) scan(g *GorgeSide) {
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, pl) {
			ob := g.E.G.Obj(id)
			if ob == nil || ob.Face() == nil {
				continue
			}
			name := ob.Face().Name
			if name != "Freestrider Commando" && name != "Rocket-Powered Goblin Glider" {
				continue
			}
			idx, from := p.entry(g, id)
			plotted := from == state.ZStack && ob.HasLatestCast && ob.LatestCastFrom == state.ZExile // a plotted free cast
			if idx < 0 || p.seen[idx] {
				continue
			}
			p.seen[idx] = true
			st := p.st
			if name == "Freestrider Commando" {
				with := ob.Counter("P1P1") > 0
				switch {
				case from == state.ZStack && plotted:
					st.plotWith, st.plotWithout = st.plotWith+b2i(with), st.plotWithout+b2i(!with)
				case from == state.ZStack:
					st.castWith, st.castWithout = st.castWith+b2i(with), st.castWithout+b2i(!with)
				default:
					st.reanimWith, st.reanimWithout = st.reanimWith+b2i(with), st.reanimWithout+b2i(!with)
				}
				continue
			}
			if from != state.ZStack {
				continue
			}
			if ob.HasLatestCast && ob.LatestCastFrom == state.ZGraveyard {
				st.gliderGrave++
			} else {
				st.gliderHand++
			}
		}
	}
}

func (p *kentercastProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	p.scan(c.G)
	name := func(x Canon) string {
		o, ok := c.G.GorgeOption(x)
		if !ok || o.Obj == 0 {
			return ""
		}
		if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	var mayhem, plot, outlets, disc, reanim, kill, creat []Canon
	graveGlider := false // a Glider discarded this turn sits in the decider's graveyard
	for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player)) {
		if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Rocket-Powered Goblin Glider" && mayhemDiscarded(c.G.E.L.Events, id) {
			graveGlider = true
		}
	}
	p.st.discarded += b2i(graveGlider)
	var enable []Canon
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		n := name(x)
		switch {
		case o.Mode == "mayhem":
			mayhem = append(mayhem, x)
			p.st.offers++
		case o.Mode == "plot" || o.Mode == "plot_cast":
			plot = append(plot, x)
		case x.Kind() == ActCast && (n == "Tormenting Voice" || n == "Thrill of Possibility"):
			outlets = append(outlets, x)
		case o.Kind == "discard" && n == "Rocket-Powered Goblin Glider":
			disc = append(disc, x)
		case x.Kind() == ActCast && n == "Dread Return":
			reanim = append(reanim, x)
		case x.Kind() == ActCast && n == "Murder":
			kill = append(kill, x)
		case x.Kind() == ActCast && n == "Freestrider Commando":
			creat = append(creat, x)
		case graveGlider && (o.Kind == "play_land" || (x.Kind() == ActCast && n == "Burning-Tree Emissary")):
			enable = append(enable, x)
		}
	}
	pickOf := func(xs []Canon, odds int) bool { return len(xs) > 0 && p.r.IntN(odds) != 0 }
	switch {
	case pickOf(mayhem, 8):
		a = mayhem[p.r.IntN(len(mayhem))]
	case pickOf(enable, 8):
		a = enable[p.r.IntN(len(enable))]
	case pickOf(disc, 8):
		a = disc[p.r.IntN(len(disc))]
	case pickOf(outlets, 4):
		a = outlets[p.r.IntN(len(outlets))]
	case pickOf(plot, 2):
		a = plot[p.r.IntN(len(plot))]
	case pickOf(reanim, 3):
		a = reanim[p.r.IntN(len(reanim))]
	case pickOf(kill, 3):
		a = kill[p.r.IntN(len(kill))]
	case pickOf(creat, 4):
		a = creat[p.r.IntN(len(creat))]
	}
	return a
}

func kentercastRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st kentercastStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("entercast")
	if err != nil {
		t.Fatal(err)
	}
	for order := 0; order < 2; order++ { // both seat orders
		md := [2][]cardset.Entry{kentercastDecks[order], kentercastDecks[1-order]}
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kentercast-a", "kentercast-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			probe := &kentercastProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x656e7463)), seen: map[int]bool{}}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
			}
			if d := res.Div; d != nil {
				diverged = true
				if report {
					lo := max(d.At-12, 0)
					t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				return
			}
		}
	}
	return
}

func TestEntercastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, stalls, st := kentercastRun(t, 300, true)
	t.Logf("%+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	entries := st.castWith + st.castWithout + st.plotWith + st.plotWithout + st.reanimWith + st.reanimWithout
	if entries < 20 {
		t.Errorf("Freestrider entries: %d, want at least 20", entries)
	}
	if st.reanimWith+st.reanimWithout < 1 {
		t.Errorf("reanimated Freestrider entries: 0, want at least 1")
	}
	if st.gliderGrave < 1 {
		t.Errorf("Glider graveyard casts: 0, want at least 1")
	}
}

// TestEntercastMutation: the enter-time spend read that ignores which
// object is entering gives a Dread-Returned Freestrider Dread Return's spend
// (no counters; gorge gives it two); wasCastFromYourGraveyard that ignores
// the cast zone triggers a hand-cast Glider.
func TestEntercastMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		name string
		flag *bool
	}{
		{"spent", &dirac.DebugSpentIgnoreReplaced},
		{"grave", &dirac.DebugWasCastYourGraveAnyZone},
	} {
		t.Run(c.name, func(t *testing.T) {
			*c.flag = true
			defer func() { *c.flag = false }()
			if diverged, _, _ := kentercastRun(t, 64, false); !diverged {
				t.Fatalf("%s: no divergence in 64 seeds per seat order", c.name)
			}
		})
	}
}
