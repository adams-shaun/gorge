package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Bolg's Company's "{T}, Sacrifice another Goblin: Add {B}{R}". gorge's planner
// census defers a sacrifice of another permanent, so Bolg's Company is a census
// gap relaxed to one free {B}{R}; a play only it pays is found by the prefix
// script, which activates it, sacrifices the first other Goblin in battlefield
// order (asked only with two or more), and floats {B}{R}. The kernel models it
// as a source kind of the tap-another search (dirac/mana_tapanother.go).
//
// Player 0's lands are Swamps and a single Mountain, so the lands alone make
// at most one {R}: Desolation of Smaug ({2}{R}{R}) is paid only with Bolg's {R}
// (the Mountain's {R} cast Bolg's Company, then pays beside it). The Goblins it
// sacrifices are Fearsome Goblin Pairs ({2}{B/R}, castable from Swamps), whose
// dies trigger (amass Goblins 4) goes on the stack before the sorcery-speed
// cast. (Every card is in the std-hob synthetic pool.)
var kbolgDecks = [2][]cardset.Entry{
	{
		{Name: "Bolg's Company", Count: 6}, {Name: "Fearsome Goblin Pair", Count: 10}, {Name: "Desolation of Smaug", Count: 3},
		{Name: "Mountain", Count: 1}, {Name: "Swamp", Count: 20},
	},
	{{Name: "Island", Count: 40}},
}

func TestBolgsCompanyLockstep(t *testing.T) {
	t.Parallel()
	tl := kbolgRun(t, 16, true, 30)
	t.Logf("games %d: Desolation of Smaug casts %d, Goblins sacrificed to Bolg's Company %d, dies triggers stacked %d",
		tl.games, tl.paid, tl.sacs, tl.dies)
	if tl.paid == 0 || tl.sacs == 0 {
		t.Fatalf("Desolation of Smaug cast %d times, %d Goblins sacrificed to Bolg's Company: the activation was not exercised", tl.paid, tl.sacs)
	}
	if tl.dies == 0 {
		t.Fatal("no sacrificed Fearsome Goblin Pair's dies trigger reached the stack")
	}
}

// TestBolgsCompanyLockstepMutation hides the sacrifice-another source in the
// kernel only: the cast Bolg's Company pays is then gorge's alone, and lockstep
// diverges.
func TestBolgsCompanyLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoSacXMana = true
	defer func() { dirac.DebugNoSacXMana = false }()
	if tl := kbolgRun(t, 16, false, 30); tl.divs == 0 {
		t.Fatal("no seed diverged without the sacrifice-another source: the scenario never needs Bolg's Company's mana")
	}
}

type kbolgTally struct {
	games, divs, paid, sacs, dies int
}

func kbolgRun(t *testing.T, seeds uint64, failOnDiv bool, maxTurns int32) kbolgTally {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-hob")
	decks, tbl := mccoyBoard(t, reg, set, kbolgDecks)
	var tl kbolgTally
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"bolg-a", "bolg-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &kbolgScenario{t: t, tl: &tl}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: maxTurns, Check: sc.check})
		CloseSide(k)
		tl.games++
		if res.Div != nil {
			tl.divs++
			if failOnDiv {
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
					res.Div.RefOptions, res.Div.KerOptions)
			}
			return tl
		}
	}
	return tl
}

type kbolgScenario struct {
	t     *testing.T
	tl    *kbolgTally
	ref   *GorgeSide
	evAt  int
	bolgT bool                 // the last event was a Bolg's Company tapping
	died  map[state.ObjID]bool // Fearsome Goblin Pairs sacrificed to Bolg's Company, their trigger not yet seen
}

func (p *kbolgScenario) Pick(c *PickCtx) Canon {
	return mulliganDonePick(p.priority, c)
}

func (p *kbolgScenario) priority(c *PickCtx) Canon {
	g := c.G.E.G
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	if a, ok := reproHandCastAction(c, g, 0, "Desolation of Smaug"); ok {
		if reproBattlefieldObj(g, "Bolg's Company", 0) == nil {
			p.t.Fatalf("turn %d: Desolation of Smaug offered with no Bolg's Company", g.Turn)
		}
		p.tl.paid++ // {R}{R}: the one Mountain makes a single {R}, so Bolg's {R} pays it
		return a
	}
	for _, name := range []string{"Bolg's Company", "Fearsome Goblin Pair"} {
		if a, ok := reproHandCastAction(c, g, 0, name); ok {
			return a
		}
	}
	return pickKind(c, ActPass)
}

// check counts, in gorge's event log, the Goblins that go to the graveyard right
// after a Bolg's Company taps (the activation's commit order: its {T}, then the
// sacrifice) and the sacrificed Fearsome Goblin Pairs whose dies trigger is pushed.
func (p *kbolgScenario) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt, p.bolgT, p.died = ref, 0, false, map[state.ObjID]bool{}
	}
	name := func(id state.ObjID) string {
		if o := ref.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.Tap:
			p.bolgT = name(e.Obj) == "Bolg's Company"
			continue
		case e.Kind == events.MoveZone && p.bolgT && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
			if n := name(e.Obj); n == "Fearsome Goblin Pair" || n == "Bolg's Company" || n == "Goblin Army" {
				p.tl.sacs++
				if n == "Fearsome Goblin Pair" {
					p.died[e.Obj] = true
				}
			}
		case e.Kind == events.TriggerPush && p.died[e.Obj]:
			p.tl.dies++
			delete(p.died, e.Obj)
		}
		p.bolgT = false
	}
	p.evAt = len(evs)
	return nil
}
