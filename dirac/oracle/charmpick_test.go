package oracle

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ChoiceRestriction$ lockstep: constructed R/W decks
// over the generated charmpick pool. The five carriers pick "one that hasn't
// been chosen this turn / this game" on a trigger (Monument to Endurance,
// Breeches, The Vision, Lita) or an activation (Three Bowls of Porridge), so
// a later offer must drop the earlier picks: the mode options are in the
// decision, and a wrong exclusion diverges on the option list. The decks add
// discard outlets and cheap Pirates and instants so a trigger repeats within
// a turn, and Cloudshift so a re-entered source offers every mode again.

var charmpickDecks = [2][]cardset.Entry{
	{ // A
		{Name: "Monument to Endurance", Count: 4}, {Name: "Breeches, Eager Pillager", Count: 1}, {Name: "Lita, Little Orphan Amphibian", Count: 1},
		{Name: "Tormenting Voice", Count: 3}, {Name: "Swab Goblin", Count: 3},
		{Name: "Goblin Trailblazer", Count: 3}, {Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Defiant Strike", Count: 1},
		{Name: "Opt", Count: 2}, {Name: "Cloudshift", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{ // B
		{Name: "The Vision", Count: 1}, {Name: "Breeches, Eager Pillager", Count: 1}, {Name: "Lita, Little Orphan Amphibian", Count: 1},
		{Name: "Monument to Endurance", Count: 2}, {Name: "Tormenting Voice", Count: 3},
		{Name: "Swab Goblin", Count: 3}, {Name: "Goblin Trailblazer", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Opt", Count: 4},
		{Name: "Defiant Strike", Count: 2}, {Name: "Cloudshift", Count: 3}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Mountain", Count: 7}, {Name: "Plains", Count: 6},
	},
}

// charmpickStats tallies what the lockstep reached (read off gorge's own
// pick log at each mode ask of a carrier).
type charmpickStats struct {
	asks      int // mode asks of a carrier
	excluded  int // ... with at least one pick already logged on the source
	repeat    int // ... a ThisTurn carrier's second or later ask in a turn
	bowlsLate int // ... Three Bowls after a turn change, still excluding
	flickered int // ... a source asked before whose log was reset by a leave and re-enter
}

type charmpickProbe struct {
	st    *charmpickStats
	base  Policy
	r     *rand.Rand
	seq   uint64
	trace []string              // the carrier asks so far: card, turn, picks logged
	first map[state.ObjID]int32 // the turn of a source's first ask
	last  map[state.ObjID]int32 // the turn of a source's latest ask
}

var charmpickCarriers = map[string]bool{
	"Monument to Endurance": true, "Breeches, Eager Pillager": true, "The Vision": true,
	"Lita, Little Orphan Amphibian": true, "Three Bowls of Porridge": true,
}

func (p *charmpickProbe) pick(c *PickCtx) Canon {
	if c.G != nil && c.G.fr != nil && c.G.fr.d != nil && string(c.G.fr.d.Kind) == "modes" && c.G.fr.d.Seq != p.seq {
		d := c.G.fr.d
		p.seq = d.Seq
		if o := c.G.E.G.Obj(d.Source); o != nil && o.Face() != nil && charmpickCarriers[o.Face().Name] {
			turn := c.G.E.G.Turn
			p.st.asks++
			bowls := o.Face().Name == "Three Bowls of Porridge"
			logged := len(o.ModeChoices) > 0
			tr := o.Face().Name + " t" + strconv.Itoa(int(turn)) + " logged"
			for _, mc := range o.ModeChoices {
				tr += " " + mc.Mode
			}
			tr += " offered"
			for _, op := range d.Options {
				tr += " " + strconv.Itoa(op.Index) + ":" + op.Label
			}
			p.trace = append(p.trace, tr)
			if logged {
				p.st.excluded++
				if !bowls {
					p.st.repeat++
				}
			}
			if first, seen := p.first[d.Source]; seen {
				if bowls && logged && turn > first {
					p.st.bowlsLate++
				}
				if !logged && (bowls || p.last[d.Source] == turn) {
					p.st.flickered++
				}
			} else {
				p.first[d.Source] = turn
			}
			p.last[d.Source] = turn
		}
	}
	a := p.base.Pick(c)
	if c.D.Kind == DPriority && p.r.IntN(4) != 0 {
		// cast, activate and play when possible, so the carriers are reached
		var acts []Canon
		for _, x := range c.D.Options {
			if k := x.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				acts = append(acts, x)
			}
		}
		if len(acts) > 0 {
			a = acts[p.r.IntN(len(acts))]
		}
	}
	return a
}

func charmpickSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmpick")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range charmpickDecks {
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
	tbl, _, _, err := set.Matchup(charmpickDecks[0], charmpickDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// charmpickRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies.
func charmpickRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st charmpickStats) {
	t.Helper()
	reg, set, tbl, decks := charmpickSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"charmpick-a", "charmpick-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &charmpickProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6368706b)),
			first: map[state.ObjID]int32{}, last: map[state.ObjID]int32{}}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d carrier asks %v", seed, probe.trace)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestCharmpickLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := charmpickRun(t, 256, true)
	t.Logf("256 games, charm picks %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.excluded, 40, "restricted asks with a mode excluded"},
		{st.repeat, 10, "second triggers in a turn"},
		{st.flickered, 3, "flickered sources"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestCharmpickMutation: ThisTurn picks that survive the turn change must
// make the lockstep diverge.
func TestCharmpickMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCharmPickNoTurnReset = true
	defer func() { dirac.DebugCharmPickNoTurnReset = false }()
	if diverged, _, _ := charmpickRun(t, 64, false); !diverged {
		t.Fatal("DebugCharmPickNoTurnReset: no divergence in 64 seeds")
	}
}
