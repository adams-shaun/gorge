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

// The may-flash lockstep (CR 702.8, K:MayFlashCost): Savage Ventmaw's attack
// trigger floats RRRGGG through combat, so gorge's hand walk offers the
// "mayflash" cast of Saproling Symbiosis ({3}{G} plus {2}) at instant
// timing, and the kernel's AkMayFlash must answer it at the same price. The
// W/U side holds Harbinger of the Tides, Mystical Tether and Rout, whose
// may-flash offers must stay withheld with an empty pool.

var mayFlashDecks = [2][]cardset.Entry{
	{
		{Name: "Savage Ventmaw", Count: 4}, {Name: "Saproling Symbiosis", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Harbinger of the Tides", Count: 4}, {Name: "Mystical Tether", Count: 4}, {Name: "Rout", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Air Elemental", Count: 3}, {Name: "Serra Angel", Count: 2},
		{Name: "Wind Drake", Count: 3},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 8},
	},
}

type mayFlashStats struct {
	casts, plainWithPool int
}

// mayFlashProbe wraps a seeded random policy biased to attack and to cast,
// counting the may-flash casts and the plain casts of a may-flash card made
// while the floating pool already paid its may-flash price (sorcery timing:
// the plain cast is the only one offered). A quarter of the games hold every
// may-flash card through combat.
type mayFlashProbe struct {
	st   *mayFlashStats
	base Policy
	r    *rand.Rand
	hold bool // never take a may-flash cast this game
}

// mayFlashCard reports whether gorge object id is a card printed with
// K:MayFlashCost.
func mayFlashCard(g *state.Game, id state.ObjID) bool {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		for _, kw := range o.Face().Keywords {
			if strings.HasPrefix(kw, "MayFlashCost") {
				return true
			}
		}
	}
	return false
}

// mayFlashPoolTotal is the mana p has floating.
func mayFlashPoolTotal(g *state.Game, p state.PlayerID) int {
	return int(g.Players[p].Pool.Total())
}

func (p *mayFlashProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	if c.D.Kind == DAttackers {
		// attack often: Savage Ventmaw's attack trigger is the pool
		var atk []Canon
		for _, x := range c.D.Options {
			if x.Kind() == ActAttack {
				atk = append(atk, x)
			}
		}
		if len(atk) > 0 && p.r.IntN(4) != 0 {
			a = atk[p.r.IntN(len(atk))]
			for _, x := range atk {
				if o := c.G.E.G.Obj(c.G.id.obj[x.Src()]); o != nil && o.Face() != nil && o.Face().Name == "Savage Ventmaw" {
					a = x // the pool's source attacks first
				}
			}
		}
		return a
	}
	if c.D.Kind != DPriority {
		return a
	}
	var mf, casts []Canon
	for _, x := range c.D.Options {
		if x.Kind() == ActCast {
			casts = append(casts, x)
		}
		if o, ok := c.G.GorgeOption(x); ok && o.Kind == "cast" && o.Mode == "mayflash" {
			mf = append(mf, x)
		}
	}
	switch {
	case len(mf) > 0 && p.hold:
		// hold the card: the pool still pays it at sorcery timing in the
		// second main phase, where only the plain cast is offered
		a = Pack(ActPass, RefNone, RefNone, 0)
	case len(mf) > 0:
		a = mf[p.r.IntN(len(mf))]
	case len(casts) > 0 && p.r.IntN(4) != 0:
		a = casts[p.r.IntN(len(casts))]
	}
	if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" {
		g := c.G.E.G
		switch {
		case o.Mode == "mayflash":
			p.st.casts++
		case o.Mode == "" && mayFlashCard(g, o.Obj) && mayFlashPoolTotal(g, state.PlayerID(c.D.Player)) >= 4:
			p.st.plainWithPool++
		}
	}
	return a
}

// mayFlashSetup builds the matchup's corpus decks and table.
func mayFlashSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mayflash")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range mayFlashDecks {
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
	tbl, _, _, err := set.Matchup(mayFlashDecks[0], mayFlashDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// mayFlashRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func mayFlashRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st mayFlashStats) {
	t.Helper()
	reg, set, tbl, decks := mayFlashSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mayflash-a", "mayflash-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &mayFlashProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6d79666c)), hold: seed%4 == 1}
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

func TestMayflashLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := mayFlashRun(t, 512, true)
	t.Logf("512 games, may-flash %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.casts < 8 {
		t.Errorf("may-flash casts: %d, want at least 8", st.casts)
	}
	if st.plainWithPool < 1 {
		t.Errorf("plain casts of a may-flash card with its may-flash price floating: %d, want at least 1", st.plainWithPool)
	}
}

// TestMayflashMutation: a may-flash cast priced at the plain mana cost must
// make the lockstep diverge.
func TestMayflashMutation(t *testing.T) {
	dirac.DebugMayFlashFree = true
	defer func() { dirac.DebugMayFlashFree = false }()
	if diverged, _, _ := mayFlashRun(t, 64, false); !diverged {
		t.Fatal("DebugMayFlashFree: no divergence in 64 seeds")
	}
}
