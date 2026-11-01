package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Mayhem lockstep (CR 702.187): constructed B/R and B/R/G decks over the
// generated mayhem pool, so every graveyard mayhem cast gorge offers (the
// card was discarded this turn: a spell or cost discard, a cleanup discard,
// a creature's upkeep discard) is answered by the kernel's mode-2 cast, and
// a Mayhem card that reached the graveyard by mill (Mulch) has no offer on
// either side.

var mayhemDecks = [2][]cardset.Entry{
	{ // A: B/R
		{Name: "Electro's Bolt", Count: 2}, {Name: "Prison Break", Count: 1}, {Name: "Raging Goblinoids", Count: 1},
		{Name: "Scarlet Spider, Kaine", Count: 2}, {Name: "Spider-Islanders", Count: 1}, {Name: "Swarm, Being of Bees", Count: 2},
		{Name: "Ultimate Green Goblin", Count: 2}, {Name: "Tormenting Voice", Count: 3}, {Name: "Cathartic Reunion", Count: 2},
		{Name: "Thrill of Possibility", Count: 3}, {Name: "Burning-Tree Emissary", Count: 3}, {Name: "Akki Rockspeaker", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B: B/R/G
		{Name: "Electro's Bolt", Count: 1}, {Name: "Prison Break", Count: 1}, {Name: "Raging Goblinoids", Count: 1},
		{Name: "Scarlet Spider, Kaine", Count: 1}, {Name: "Spider-Islanders", Count: 2}, {Name: "Swarm, Being of Bees", Count: 2},
		{Name: "Ultimate Green Goblin", Count: 1}, {Name: "Tormenting Voice", Count: 2}, {Name: "Cathartic Reunion", Count: 2},
		{Name: "Thrill of Possibility", Count: 2}, {Name: "Mulch", Count: 3}, {Name: "Satyr Wayfinder", Count: 1},
		{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Lotus Cobra", Count: 2}, {Name: "Mardu Warshrieker", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Fire Elemental", Count: 1}, {Name: "Disfigure", Count: 1},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

type mayhemStats struct {
	casts, offers int
	milledGames   int // games where a milled Mayhem card sat in a graveyard with no offer
}

// mayhemMilled reports whether the player's graveyard holds a Mayhem card
// that was not discarded since the last turn change (gorge
// mayhemDiscardedThisTurn's window), and none of options casts it for mayhem.
func mayhemMilled(g *GorgeSide, pl uint8, opts []Canon) bool {
	e := g.E
	for _, id := range e.G.Zone(state.ZGraveyard, state.PlayerID(pl)) {
		ob := e.G.Obj(id)
		if ob == nil || ob.Face() == nil || !mayhemNames[ob.Face().Name] {
			continue
		}
		// milled: the card's last move put it into the graveyard from the
		// library (Mulch), and no discard of it since the last turn change
		milled := false
		for i := len(e.L.Events) - 1; i >= 0; i-- {
			ev := e.L.Events[i]
			if ev.Kind == events.MoveZone && ev.Obj == id {
				milled = ev.From == state.ZLibrary && ev.To == state.ZGraveyard
				break
			}
		}
		if !milled {
			continue
		}
		offered := false
		for _, x := range opts {
			if o, ok := g.GorgeOption(x); ok && o.Obj == id && o.Mode == "mayhem" {
				offered = true
			}
		}
		if !offered {
			return true
		}
	}
	return false
}

// mayhemDiscarded reports a discard of id since the last turn change.
func mayhemDiscarded(evs []events.Event, id state.ObjID) bool {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == events.TurnChange {
			return false
		}
		if evs[i].Kind == events.MoveZone && evs[i].Obj == id && events.IsDiscard(evs[i]) {
			return true
		}
	}
	return false
}

var mayhemNames = map[string]bool{
	"Electro's Bolt": true, "Prison Break": true, "Raging Goblinoids": true, "Scarlet Spider, Kaine": true,
	"Spider-Islanders": true, "Swarm, Being of Bees": true, "Ultimate Green Goblin": true,
}

// mayhemOutlets are the pool's spells that discard a card the caster picks.
var mayhemOutlets = map[string]bool{"Tormenting Voice": true, "Cathartic Reunion": true, "Thrill of Possibility": true}

// mayhemMana are the pool's cards that float mana on entering or on a land drop.
var mayhemMana = map[string]bool{"Burning-Tree Emissary": true, "Akki Rockspeaker": true, "Lotus Cobra": true, "Mardu Warshrieker": true}

// mayhemProbe wraps a seeded random policy biased to cast, counting the
// mayhem casts gorge offers and takes.
type mayhemProbe struct {
	st     *mayhemStats
	base   Policy
	r      *rand.Rand
	milled bool
}

func (p *mayhemProbe) pick(c *PickCtx) Canon {
	var casts []Canon
	for _, a := range c.D.Options {
		if a.Kind() == ActCast {
			casts = append(casts, a)
		}
	}
	a := p.base.Pick(c)
	if c.G != nil {
		if !p.milled && mayhemMilled(c.G, c.D.Player, c.D.Options) {
			p.milled = true
			p.st.milledGames++
		}
		var may []Canon
		for _, x := range c.D.Options {
			if o, ok := c.G.GorgeOption(x); ok && o.Mode == "mayhem" {
				may = append(may, x)
			}
		}
		if len(may) > 0 {
			p.st.offers++
		}
		eligible := false // a Mayhem card discarded this turn sits in the decider's graveyard
		for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player)) {
			if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && mayhemNames[ob.Face().Name] && mayhemDiscarded(c.G.E.L.Events, id) {
				eligible = true
			}
		}
		var enable []Canon // gorge prices no mayhem mode: the cast needs floating mana
		for _, x := range c.D.Options {
			if o, ok := c.G.GorgeOption(x); ok && eligible && (o.Kind == "play_land" || o.Kind == "cast") {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && (mayhemMana[ob.Face().Name] || o.Kind == "play_land") {
					enable = append(enable, x)
				}
			}
		}
		var disc, outlets []Canon
		for _, x := range casts {
			if o, ok := c.G.GorgeOption(x); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && mayhemOutlets[ob.Face().Name] {
					outlets = append(outlets, x)
				}
			}
		}
		for _, x := range c.D.Options {
			if o, ok := c.G.GorgeOption(x); ok && o.Kind == "discard" {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && mayhemNames[ob.Face().Name] {
					disc = append(disc, x)
				}
			}
		}
		switch {
		case len(may) > 0 && p.r.IntN(8) != 0:
			a = may[p.r.IntN(len(may))]
		case len(enable) > 0 && p.r.IntN(8) != 0:
			a = enable[p.r.IntN(len(enable))] // float mana for the cast
		case len(outlets) > 0 && p.r.IntN(4) != 0:
			a = outlets[p.r.IntN(len(outlets))] // a discard outlet
		case len(disc) > 0 && p.r.IntN(8) != 0:
			a = disc[p.r.IntN(len(disc))] // discard a Mayhem card
		case len(casts) > 0 && p.r.IntN(4) != 0:
			a = casts[p.r.IntN(len(casts))]
		}
		if a.Kind() == ActCast {
			if o, ok := c.G.GorgeOption(a); ok && o.Mode == "mayhem" {
				p.st.casts++
			}
		}
	}
	return a
}

// mayhemSetup builds the matchup's corpus decks and table.
func mayhemSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mayhem")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range mayhemDecks {
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
	tbl, _, _, err := set.Matchup(mayhemDecks[0], mayhemDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// mayhemRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func mayhemRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st mayhemStats) {
	t.Helper()
	reg, set, tbl, decks := mayhemSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mayhem-a", "mayhem-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &mayhemProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6d617968))}
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

func TestMayhemLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := mayhemRun(t, 256, true)
	t.Logf("256 games, mayhem %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.casts, 30, "mayhem casts"},
		{st.milledGames, 10, "games with a milled Mayhem card in a graveyard and no offer"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestMayhemMutation: an offer that ignores the discarded-this-turn bit
// must make the lockstep diverge (a milled Mayhem card is offered).
func TestMayhemMutation(t *testing.T) {
	dirac.DebugMayhemAnyGrave = true
	defer func() { dirac.DebugMayhemAnyGrave = false }()
	if diverged, _, _ := mayhemRun(t, 64, false); !diverged {
		t.Fatal("DebugMayhemAnyGrave: no divergence in 64 seeds")
	}
}
