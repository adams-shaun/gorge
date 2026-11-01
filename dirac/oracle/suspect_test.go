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

// The suspected lockstep (CR 701.60): constructed B/R and W/U/R decks over
// the generated suspect pool, so every suspect and un-suspect carrier, the
// menace and can't-block it grants, and the designation staying through a
// control change (Act of Treason's, Caught Red-Handed's revert at end of
// turn) are compared with gorge, the designation itself through FSuspected.

var suspectDecks = [2][]cardset.Entry{
	{ // A: B/R
		{Name: "Barbed Servitor", Count: 2}, {Name: "Caught Red-Handed", Count: 2}, {Name: "Clandestine Meddler", Count: 2},
		{Name: "Convenient Target", Count: 2}, {Name: "Deadly Complication", Count: 2}, {Name: "It Doesn't Add Up", Count: 1},
		{Name: "Person of Interest", Count: 1}, {Name: "Presumed Dead", Count: 2}, {Name: "Repeat Offender", Count: 2},
		{Name: "Rubblebelt Braggart", Count: 2}, {Name: "Rune-Brand Juggler", Count: 1}, {Name: "J. Jonah Jameson", Count: 1},
		{Name: "Act of Treason", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Disfigure", Count: 1},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 7},
	},
	{ // B: W/U/R
		{Name: "Absolving Lammasu", Count: 2}, {Name: "Eliminate the Impossible", Count: 2}, {Name: "Person of Interest", Count: 2},
		{Name: "J. Jonah Jameson", Count: 1}, {Name: "Caught Red-Handed", Count: 2}, {Name: "Act of Treason", Count: 2},
		{Name: "Rubblebelt Braggart", Count: 2}, {Name: "Savannah Lions", Count: 3}, {Name: "Air Elemental", Count: 1},
		{Name: "Fire Elemental", Count: 1}, {Name: "Hill Giant", Count: 2}, {Name: "Convenient Target", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 9},
	},
}

type suspectStats struct {
	suspects, unsuspects, ctrlStays, blockBarred int
}

// suspectProbe wraps a seeded random policy biased to cast. It reads gorge's
// Suspected flag between picks: a flag that rose is a suspect; one that fell
// on a permanent still on the battlefield is an un-suspect, or a control
// change's end of the designation when its controller changed. A turn whose
// declare-blockers step finds the defender with an untapped suspected
// creature counts a block the creature could not make.
type suspectProbe struct {
	st       *suspectStats
	base     Policy
	r        *rand.Rand
	prev     map[state.ObjID]suspectSeen
	t1       Ref // the creature picked for Deadly Complication's second target slot (RefNone: none)
	barTurn  int32
	barTurnP bool
}

type suspectSeen struct {
	on   bool
	ctrl state.PlayerID
}

func (p *suspectProbe) pick(c *PickCtx) Canon {
	if g := c.G; g != nil {
		p.observe(g.E.G)
	}
	a := p.base.Pick(c)
	var casts []Canon
	for _, x := range c.D.Options {
		if x.Kind() == ActCast {
			casts = append(casts, x)
		}
	}
	if len(casts) > 0 && p.r.IntN(4) != 0 {
		a = casts[p.r.IntN(len(casts))]
	}
	return p.keepTargetsApart(c, a)
}

// keepTargetsApart steers Deadly Complication's first target slot off the
// creature already picked for its second. A creature destroyed by the first
// mode and then given the second mode's +1/+1 counter keeps that counter in
// gorge's graveyard, and a later return to the battlefield carries it back
// (gorge effPutCounter's zone-agnostic loop and events apply_zone.go's
// counter reset): a gorge finding (CR 400.7), not a kernel difference, so the
// pool's games never reach it.
func (p *suspectProbe) keepTargetsApart(c *PickCtx, a Canon) Canon {
	if a.Kind() != ActTarget {
		p.t1 = RefNone
		return a
	}
	if a.Mode() == 1 {
		p.t1 = a.Tgt()
		return a
	}
	if p.t1 != RefNone && a.Tgt() == p.t1 {
		for _, x := range c.D.Options {
			if x.Kind() == ActTarget && x.Mode() == a.Mode() && x.Tgt() != p.t1 {
				return x
			}
		}
	}
	return a
}

func (p *suspectProbe) observe(g *state.Game) {
	if p.prev == nil {
		p.prev = map[state.ObjID]suspectSeen{}
	}
	for i := range g.Objs {
		o := &g.Objs[i]
		now := o.Zone == state.ZBattlefield && o.Suspected
		was := p.prev[o.ID]
		switch {
		case now && !was.on:
			p.st.suspects++
		case now && was.on && o.Controller != was.ctrl:
			p.st.ctrlStays++
		case !now && was.on && o.Zone == state.ZBattlefield:
			p.st.unsuspects++
		}
		p.prev[o.ID] = suspectSeen{now, o.Controller}
		if now && g.Step == state.StepDeclareBlockers && o.Controller != g.Active && !o.Tapped && (p.barTurn != g.Turn || !p.barTurnP) {
			p.barTurn, p.barTurnP = g.Turn, true
			p.st.blockBarred++
		}
	}
}

// suspectSetup builds the matchup's corpus decks and table.
func suspectSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("suspect")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range suspectDecks {
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
	tbl, _, _, err := set.Matchup(suspectDecks[0], suspectDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// suspectRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func suspectRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st suspectStats) {
	t.Helper()
	reg, set, tbl, decks := suspectSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"suspect-a", "suspect-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &suspectProbe{st: &st, t1: RefNone, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x7375737063))}
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

func TestSuspectLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := suspectRun(t, 256, true)
	t.Logf("256 games, suspect %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.suspects, 40, "suspects"},
		{st.unsuspects, 10, "un-suspects"},
		{st.blockBarred, 10, "turns a suspected creature could not block"},
		{st.ctrlStays, 3, "control changes a suspect stayed through"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestSuspectMutation: a canBlock that ignores the designation must make
// the lockstep diverge.
func TestSuspectMutation(t *testing.T) {
	dirac.DebugSuspectCanBlock = true
	defer func() { dirac.DebugSuspectCanBlock = false }()
	if diverged, _, _ := suspectRun(t, 64, false); !diverged {
		t.Fatal("DebugSuspectCanBlock: no divergence in 64 seeds")
	}
}
