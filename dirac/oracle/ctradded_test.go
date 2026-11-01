package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Count$CountersAddedThisTurn lockstep: Lasting Tarfire's
// and Fractal Tender's end-step gates, Beast's flying gate and Summon:
// Brynhildr's MayPlay gate over a pool of counter makers (a +1/+1 spell, a
// proliferate card, a flicker), so every counter placement, the "You" of the
// head and the turn reset are compared with gorge's ledger.

var ctrAddedDecks = [2][]cardset.Entry{
	{ // A: G/U/R
		{Name: "Lasting Tarfire", Count: 3}, {Name: "Beast, Erudite Aerialist", Count: 3}, {Name: "Fractal Tender", Count: 3},
		{Name: "Battlegrowth", Count: 3}, {Name: "Hunter's Edge", Count: 2},
		{Name: "Thrummingbird", Count: 2}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 5},
	},
	{ // B: G/U/W, the opponent that puts counters on its own board
		{Name: "Beast, Erudite Aerialist", Count: 2}, {Name: "Fractal Tender", Count: 3}, {Name: "Battlegrowth", Count: 3},
		{Name: "Hunter's Edge", Count: 2}, {Name: "Thrummingbird", Count: 2}, {Name: "Flux Channeler", Count: 3},
		{Name: "Cloudshift", Count: 3}, {Name: "Lord Jyscal Guado", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 5},
	},
}

type ctrAddedStats struct {
	gateTrue, gateFalse, oppOnly, beastFly int
}

// ctrAddedProbe wraps a seeded random policy and samples gorge's own ledger
// at every decision; the last sample of a turn is that turn's tally.
type ctrAddedProbe struct {
	st   *ctrAddedStats
	base Policy
	turn int
	// the current turn's latest sample: active put, other put, a Beast got a +1/+1
	act, oth, fly bool
}

func (p *ctrAddedProbe) flush() {
	if p.turn == 0 {
		return
	}
	if p.act {
		p.st.gateTrue++
	} else {
		p.st.gateFalse++
	}
	if p.oth && !p.act {
		p.st.oppOnly++
	}
	if p.fly {
		p.st.beastFly++
	}
}

func (p *ctrAddedProbe) pick(c *PickCtx) Canon {
	if g := c.G; g != nil {
		e := g.E
		if t := int(e.G.Turn); t != p.turn {
			p.flush()
			p.turn, p.act, p.oth, p.fly = t, false, false, false
		}
		act := e.G.Active
		put := func(pl state.PlayerID) bool {
			return e.CountersAddedThisTurn("Any", "You", "Creature", effects.SpecContext{You: pl}) > 0
		}
		p.act, p.oth = put(act), put(1-act)
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Zone != state.ZBattlefield || o.Card == nil || len(o.Card.Faces) == 0 || o.Card.Faces[0].Name != "Beast, Erudite Aerialist" {
				continue
			}
			if e.CountersAddedThisTurn("P1P1", "You", "Card.Self", effects.SpecContext{You: o.Controller, Source: o.ID}) > 0 {
				p.fly = true
			}
		}
	}
	return p.base.Pick(c)
}

func ctrAddedSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ctradded")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ctrAddedDecks {
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
	tbl, _, _, err := set.Matchup(ctrAddedDecks[0], ctrAddedDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// ctrAddedRun plays seeds 1..seeds, reporting the first divergence as a test
// error when report is set (the mutation run expects one and stays quiet).
func ctrAddedRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st ctrAddedStats) {
	t.Helper()
	reg, set, tbl, decks := ctrAddedSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ctradded-a", "ctradded-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &ctrAddedProbe{st: &st, base: RandomPolicy(seed)}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		probe.flush()
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
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

func TestCtraddedLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := ctrAddedRun(t, 256, true)
	t.Logf("256 games, %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.gateTrue, 40, "turns the active player put a counter on a creature (end-step gates true)"},
		{st.gateFalse, 40, "turns without one (gates false)"},
		{st.oppOnly, 10, "turns only the opponent put a counter"},
		{st.beastFly, 10, "turns a Beast got a +1/+1 counter (flying)"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestCtraddedMutation: a record that survives the turn boundary must make
// the lockstep diverge.
func TestCtraddedMutation(t *testing.T) {
	dirac.DebugCtrAddedNoTurnReset = true
	defer func() { dirac.DebugCtrAddedNoTurnReset = false }()
	if diverged, _, _ := ctrAddedRun(t, 64, false); !diverged {
		t.Fatal("DebugCtrAddedNoTurnReset: no divergence in 64 seeds")
	}
}
