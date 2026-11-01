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

// The self flash lockstep (gorge withSelfStatics): a card's own
// `CastWithFlash | ValidCard$ Card.Self | EffectZone$ All` static lets it be
// cast at instant speed from hand while its IsPresent$ gate holds for the
// caster (Illusion Spinners: a Faerie; Bard's Company: a Human; Serpent of
// the Pass: three Lessons in the graveyard). Captain Mar-Vell's
// battlefield-only grant (no EffectZone$) never grants its own card flash.

var selfFlashDecks = [2][]cardset.Entry{
	{ // A: W/U Faeries and Humans
		{Name: "Illusion Spinners", Count: 3}, {Name: "Bard's Company", Count: 3}, {Name: "Captain Mar-Vell, Space-Born", Count: 2},
		{Name: "Cloud Sprite", Count: 5},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Glory Seeker", Count: 2}, {Name: "Border Guard", Count: 2},
		{Name: "Cancel", Count: 2}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Raise the Alarm", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7},
	},
	{ // B: U/G Lessons and self-mill
		{Name: "Serpent of the Pass", Count: 4}, {Name: "Environmental Sciences", Count: 4}, {Name: "Expanded Anatomy", Count: 3},
		{Name: "Accumulate Wisdom", Count: 3}, {Name: "Mulch", Count: 3}, {Name: "Illusion Spinners", Count: 2}, {Name: "Moon Sprite", Count: 2},
		{Name: "Cancel", Count: 2}, {Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 6},
	},
}

type selfFlashStats struct {
	casts   int // off-sorcery-timing casts of a self flash carrier
	blocked int // priority windows where a carrier in hand was not offered, its gate failing off-timing
	byName  map[string]int
	marvell int // off-timing creature casts offered beside Captain Mar-Vell's battlefield grant
}

// selfFlashProbe wraps a seeded random policy biased to take an off-timing
// carrier cast when one is offered, and counts the casts and the windows
// where a carrier was withheld by its gate.
type selfFlashProbe struct {
	st   *selfFlashStats
	base Policy
	r    *rand.Rand
}

func hasFaceType(f *cards.Face, t string) bool {
	for _, x := range f.Types {
		if x == t {
			return true
		}
	}
	return false
}

// selfFlashGate is the carrier's IsPresent$ gate for player p, read from
// gorge's state independently of the kernel.
func selfFlashGate(g *state.Game, p state.PlayerID, name string) (carrier, holds bool) {
	count := func(z state.Zone, ty string) int {
		n := 0
		for _, id := range g.Zone(z, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && hasFaceType(o.Face(), ty) {
				n++
			}
		}
		return n
	}
	switch name {
	case "Illusion Spinners":
		return true, count(state.ZBattlefield, "Faerie") >= 1
	case "Bard's Company":
		return true, count(state.ZBattlefield, "Human") >= 1
	case "Serpent of the Pass":
		return true, count(state.ZGraveyard, "Lesson") >= 3
	}
	return false, false
}

func (p *selfFlashProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	d := c.G.Pending()
	if d == nil || d.Kind != "priority" {
		return a
	}
	g := c.G.E.G
	off := !(g.Step.IsMain() && g.Active == g.Priority && len(g.Stack) == 0)
	if !off {
		return a
	}
	var offered []Canon                 // option order
	offeredID := map[state.ObjID]bool{} // membership only, never ranged
	for _, x := range c.D.Options {
		if x.Kind() != ActCast {
			continue
		}
		if o, ok := c.G.GorgeOption(x); ok {
			if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
				if carrier, _ := selfFlashGate(g, g.Priority, ob.Face().Name); carrier {
					offered = append(offered, x)
					offeredID[o.Obj] = true
				} else if hasFaceType(ob.Face(), "Creature") {
					p.st.marvell++ // a creature spell at instant speed: only a battlefield grant allows it
				}
			}
		}
	}
	for _, id := range g.Zone(state.ZHand, g.Priority) {
		ob := g.Obj(id)
		if ob == nil || ob.Face() == nil {
			continue
		}
		if carrier, holds := selfFlashGate(g, g.Priority, ob.Face().Name); carrier && !holds {
			if !offeredID[id] {
				p.st.blocked++
			}
		}
	}
	if len(offered) > 0 && p.r.IntN(4) != 0 {
		a = offered[p.r.IntN(len(offered))]
	}
	if a.Kind() == ActCast {
		for _, x := range offered {
			if x == a {
				p.st.casts++
				if o, ok := c.G.GorgeOption(a); ok {
					if p.st.byName == nil {
						p.st.byName = map[string]int{}
					}
					p.st.byName[g.Obj(o.Obj).Face().Name]++
				}
			}
		}
	}
	return a
}

// selfFlashSetup builds the matchup's corpus decks and table.
func selfFlashSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("selfflash")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range selfFlashDecks {
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
	tbl, _, _, err := set.Matchup(selfFlashDecks[0], selfFlashDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// selfFlashRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func selfFlashRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st selfFlashStats) {
	t.Helper()
	reg, set, tbl, decks := selfFlashSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"selfflash-a", "selfflash-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &selfFlashProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x73666c73))}
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

func TestSelfflashLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := selfFlashRun(t, 256, true)
	t.Logf("256 games, self flash %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, name := range []string{"Illusion Spinners", "Bard's Company", "Serpent of the Pass"} {
		if st.byName[name] < 3 {
			t.Errorf("%s: %d off-timing casts, want at least 3", name, st.byName[name])
		}
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.casts, 30, "off-sorcery-timing carrier casts"},
		{st.marvell, 1, "off-timing creature casts under Captain Mar-Vell"},
		{st.blocked, 10, "windows with a carrier withheld by its gate"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestSelfflashMutation: selfFlash reading false must make the lockstep
// diverge (gorge offers the carrier at instant speed, the kernel does not).
func TestSelfflashMutation(t *testing.T) {
	dirac.DebugSelfFlashOff = true
	defer func() { dirac.DebugSelfFlashOff = false }()
	if diverged, _, _ := selfFlashRun(t, 64, false); !diverged {
		t.Fatal("DebugSelfFlashOff: no divergence in 64 seeds")
	}
}
