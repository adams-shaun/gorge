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

// The Retrace lockstep (CR 702.81a): constructed decks over the generated
// retrace pool, so every graveyard retrace cast gorge offers (the printed
// mana cost plus the additional discard-a-land cost, offered only when the
// floating pool pays the printed base) is answered by the kernel's mode-2
// cast of retraceTgt, the land discard is asked and charged identically, and
// the spell rests in its owner's graveyard on resolution.

var retraceDecks = [2][]cardset.Entry{
	{ // A: U/R/G with the red rituals that float the expensive printed bases
		{Name: "Call the Skybreaker", Count: 3}, {Name: "Spitting Image", Count: 3},
		{Name: "Throes of Chaos", Count: 2}, {Name: "Formless Genesis", Count: 2},
		{Name: "Worm Harvest", Count: 1},
		{Name: "Inner Fire", Count: 2}, {Name: "Seething Song", Count: 2},
		{Name: "Rite of Flame", Count: 1},
		{Name: "Tormenting Voice", Count: 2},
		{Name: "Manamorphose", Count: 3},
		{Name: "Satyr Wayfinder", Count: 1},
		{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Lotus Cobra", Count: 1},
		{Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 5},
	},
	{ // B: B/R/G/U with the black rituals and the blue ones' landfall pair
		{Name: "Raven's Crime", Count: 2}, {Name: "Flame Jab", Count: 2},
		{Name: "Oona's Grace", Count: 2}, {Name: "Throes of Chaos", Count: 2},
		{Name: "Worm Harvest", Count: 2},
		{Name: "Dark Ritual", Count: 2}, {Name: "Seething Song", Count: 2},
		{Name: "Tormenting Voice", Count: 2}, {Name: "Thrill of Possibility", Count: 1},
		{Name: "Mulch", Count: 2},
		{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Lotus Cobra", Count: 2},
		{Name: "Swamp", Count: 5}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 4},
		{Name: "Forest", Count: 2},
	},
}

// retraceList are the pool's printed Retrace cards, the brief's six carriers
// minus Reality Scramble (refused: Card.sharesCardTypeWith RememberedLKI in
// its DigUntil filter) plus the three classmates the corpus survey found.
var retraceList = []string{
	"Call the Skybreaker", "Spitting Image", "Throes of Chaos",
	"Formless Genesis", "Worm Harvest", "Oona's Grace", "Raven's Crime", "Flame Jab",
}

// retraceNames are the pool's printed Retrace cards.
var retraceNames = map[string]bool{}

func init() {
	for _, n := range retraceList {
		retraceNames[n] = true
	}
}

// retraceMana are the pool's creatures that float mana when they enter or
// through an activation; the retrace cast's printed base is offered only
// when the floating pool pays it.
var retraceMana = map[string]bool{
	"Burning-Tree Emissary": true, "Lotus Cobra": true, "Akki Rockspeaker": true,
	"Mardu Warshrieker": true,
}

// retraceOutlets are the pool's spells that discard a card the caster picks.
var retraceOutlets = map[string]bool{
	"Tormenting Voice": true, "Thrill of Possibility": true, "Cathartic Reunion": true,
}

// retraceBodies are the pool's creatures: Spitting Image targets one, so the
// board must hold one when its printed base floats.
var retraceBodies = map[string]bool{
	"Hill Giant": true, "Burning-Tree Emissary": true, "Lotus Cobra": true,
}

// retraceRituals are the pool's mana rituals: under both oracle surfaces
// mana abilities are never options, so the printed bases float only through
// the rituals' casts and the ETB producers' triggers.
var retraceRituals = map[string]bool{
	"Dark Ritual": true, "Rite of Flame": true, "Seething Song": true,
}

type retraceStats struct {
	casts, offers int
	perCard       map[string]int
	offersCard    map[string]int
}

// retraceProbe wraps a seeded random policy biased to the retrace cast, to
// floating mana, and to the outlets that bin a Retrace card, counting the
// retrace casts gorge offers and takes.
type retraceProbe struct {
	st   *retraceStats
	base Policy
	r    *rand.Rand
}

func (p *retraceProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	ready := false // a printed Retrace card sits in the decider's graveyard
	for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player)) {
		if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && retraceNames[ob.Face().Name] {
			ready = true
		}
	}
	var ret, enable, outlets, disc, rit, bodies []Canon
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		ob := c.G.E.G.Obj(o.Obj)
		name := ""
		if ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		switch {
		case o.Mode == "retrace":
			ret = append(ret, x)
			if ob != nil && ob.Face() != nil {
				p.st.offersCard[ob.Face().Name]++
			}
		case ready && o.Kind == "ability" && (retraceMana[name] || ob != nil && ob.Face() != nil && ob.Face().IsLand()):
			enable = append(enable, x) // float mana for the printed-base cast
		case ready && o.Kind == "play_land":
			enable = append(enable, x)
		case o.Kind == "cast" && retraceRituals[name]:
			rit = append(rit, x) // float the printed base's mana
		case o.Kind == "cast" && retraceBodies[name]:
			bodies = append(bodies, x) // Spitting Image needs a target creature
		case o.Kind == "cast" && retraceOutlets[name]:
			outlets = append(outlets, x) // bin a Retrace card
		case o.Kind == "discard" && retraceNames[name]:
			disc = append(disc, x)
		}
	}
	if len(ret) > 0 {
		p.st.offers++
	}
	switch {
	case len(ret) > 0 && p.r.IntN(16) != 0:
		a = ret[p.r.IntN(len(ret))] // the offer is up: the pool pays the printed base
	case ready && len(enable) > 0 && p.r.IntN(10) != 0:
		a = enable[p.r.IntN(len(enable))] // stockpile the printed base's mana
	case ready && len(rit) > 0 && p.r.IntN(6) != 0:
		a = rit[p.r.IntN(len(rit))]
	case !ready && len(bodies) > 0 && p.r.IntN(3) != 0:
		a = bodies[p.r.IntN(len(bodies))]
	case !ready && len(outlets) > 0 && p.r.IntN(4) != 0:
		a = outlets[p.r.IntN(len(outlets))]
	case !ready && len(disc) > 0 && p.r.IntN(8) != 0:
		a = disc[p.r.IntN(len(disc))]
	}
	if a.Kind() == ActCast {
		if o, ok := c.G.GorgeOption(a); ok && o.Mode == "retrace" {
			p.st.casts++
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				p.st.perCard[ob.Face().Name]++
			}
		}
	}
	return a
}

// retraceSetup builds the matchup's corpus decks and table.
func retraceSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("retrace")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range retraceDecks {
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
	tbl, _, _, err := set.Matchup(retraceDecks[0], retraceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// retraceRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func retraceRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st retraceStats) {
	t.Helper()
	reg, set, tbl, decks := retraceSetup(t)
	st.perCard = map[string]int{}
	st.offersCard = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"retrace-a", "retrace-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &retraceProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x72657472))}
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

func TestRetraceLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := retraceRun(t, 1024, true)
	t.Logf("1024 games, retrace %+v, %d divergences, %d stalls, offers by card %v", st, b2i(div), stalls, st.offersCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.offers, 20, "retrace offers"},
		{st.casts, 10, "retrace casts"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
	for _, n := range retraceList {
		if st.perCard[n] == 0 {
			t.Errorf("%s: never retrace-cast in the lockstep", n)
		}
	}
}

// TestRetraceMutation: a retrace cast that charges no discard must make the
// lockstep diverge (the land is never asked for or discarded).
func TestRetraceMutation(t *testing.T) {
	dirac.DebugRetraceFree = true
	defer func() { dirac.DebugRetraceFree = false }()
	if diverged, _, _ := retraceRun(t, 32, false); !diverged {
		t.Fatal("DebugRetraceFree: no divergence in 32 seeds")
	}
}
