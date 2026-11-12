package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Ramos, Dragon Engine's "Remove five +1/+1 counters from Ramos: Add
// {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}. Activate only once each turn." gorge's planner
// census cannot pay a SubCounter cost, so Ramos is a census gap relaxed to one
// free alternative holding the whole production; a play only it pays is found
// by the prefix script, which activates Ramos first (potentialPrefixScript).
// The kernel models it as the counter-cost gap (dirac/mana_countercost.go).
//
// Player 0 holds only Plains: white spells grow Ramos (one counter each), and
// Elfsworn Giant ({3}{G}{G}) and Shivan Dragon ({4}{R}{R}) can be paid only by
// Ramos's ten mana. Lockstep checks every decision against gorge; the scenario
// also checks the positions named in the ticket: with five counters and the
// activation unused the Elemental cast is offered (the activation removes
// five counters), and with four counters it is not offered.
var kramosDecks = [2][]cardset.Entry{
	{
		{Name: "Ramos, Dragon Engine", Count: 4}, {Name: "Savannah Lions", Count: 10}, {Name: "Healer's Hawk", Count: 10},
		{Name: "Progenitus", Count: 2},
		{Name: "Plains", Count: 14},
	},
	{{Name: "Island", Count: 40}},
}

// kramosOffColour are player 0's spells only Ramos's mana pays: Progenitus
// ({W}{W}{U}{U}{B}{B}{R}{R}{G}{G}) costs exactly the ten mana, and no Plains
// can make the {U}{U}{B}{B}{R}{R}{G}{G}.
var kramosOffColour = []string{"Progenitus"}

func TestRamosDragonEngineLockstep(t *testing.T) {
	t.Parallel()
	tl := kramosRun(t, 24, true, 30)
	t.Logf("games %d: off-colour casts %d (counters removed %d), refused at four counters %d, refused after the turn's use %d",
		tl.games, tl.paid, tl.removed, tl.four, tl.used)
	if tl.paid == 0 || tl.removed != tl.paid {
		t.Fatalf("Ramos paid %d casts and removed five counters %d times: the activation was not exercised", tl.paid, tl.removed)
	}
	if tl.four == 0 {
		t.Fatal("no position with four counters and an off-colour spell in hand")
	}
}

// TestRamosDragonEngineLockstepMutation hides the counter-cost gap in the kernel
// only: the cast Ramos pays is then gorge's alone, and lockstep diverges.
func TestRamosDragonEngineLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoCtrMana = false }()
	if tl := kramosRun(t, 24, false, 30); tl.divs == 0 {
		t.Fatal("no seed diverged without the counter-cost gap: the scenario never needs Ramos's mana")
	}
}

type kramosTally struct {
	games, divs, paid, removed, four, used int
}

func kramosRun(t *testing.T, seeds uint64, failOnDiv bool, maxTurns int32) kramosTally {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	decks, tbl := mccoyBoard(t, reg, set, kramosDecks)
	var tl kramosTally
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"ramos-a", "ramos-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &kramosScenario{t: t, tl: &tl, usedTurn: -1}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: maxTurns, Check: sc.check})
		CloseSide(k)
		t.Logf("DBG seed %d: decisions %d stall %q ramos %v", seed, len(res.Actions), res.Stall, sc.dbg)
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

type kramosScenario struct {
	t        *testing.T
	tl       *kramosTally
	usedTurn int32
	ref      *GorgeSide
	evAt     int
	dbg      []int32
}

func (p *kramosScenario) Pick(c *PickCtx) Canon {
	return mulliganDonePick(p.priority, c)
}

func (p *kramosScenario) priority(c *PickCtx) Canon {
	g := c.G.E.G
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	ramos := reproBattlefieldObj(g, "Ramos, Dragon Engine", 0)
	n := int32(0)
	if ramos != nil {
		n = ramos.Counter("P1P1")
	}
	if ramos != nil {
		p.dbg = append(p.dbg, n)
	}
	var off Canon
	held, offered := false, false
	for _, name := range kramosOffColour {
		if kramosInHand(g, name) {
			held = true
		}
		if a, ok := reproHandCastAction(c, g, 0, name); ok && !offered {
			off, offered = a, true
		}
	}
	// a sorcery-speed position: some creature cast is offered (a white one)
	sorcery := false
	for _, name := range []string{"Savannah Lions", "Healer's Hawk", "Ramos, Dragon Engine"} {
		if _, ok := reproHandCastAction(c, g, 0, name); ok {
			sorcery = true
		}
	}
	// An off-colour cast needs {G}{G} or {R}{R}: only Ramos's activation can
	// make those. Within the activation's turn the leftover float may still
	// pay a second cast at fewer counters, so the no-activation invariant
	// holds only before the turn's use; once-per-turn and the float's limits
	// are the lockstep's own comparison (a wrongly repeated gap adds options
	// gorge does not list).
	usedTurn := p.usedTurn == g.Turn
	switch {
	case offered && (ramos == nil || (n < 5 && !usedTurn)):
		p.t.Fatalf("turn %d: an off-colour cast is offered with Ramos at %d counters (used this turn: %v)", g.Turn, n, usedTurn)
	case held && ramos != nil && n == 4 && !usedTurn && sorcery:
		p.tl.four++
	case held && ramos != nil && usedTurn && sorcery:
		p.tl.used++
	}
	if offered {
		p.usedTurn = g.Turn
		if !usedTurn && n >= 5 {
			p.tl.paid++ // a cast only Ramos's activation pays (check pairs it with the -5)
		}
		return off
	}
	for _, name := range []string{"Ramos, Dragon Engine", "Ingenious Leonin", "Healer's Hawk", "Savannah Lions"} {
		if name == "Ramos, Dragon Engine" && ramos != nil {
			continue // (the legend rule would ask)
		}
		if a, ok := reproHandCastAction(c, g, 0, name); ok {
			return a
		}
	}
	return pickKind(c, ActPass)
}

func kramosInHand(g *state.Game, name string) bool {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone == state.ZHand && o.Owner == 0 && o.Card != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// check counts gorge's removals of five +1/+1 counters from Ramos (the
// activation's cost).
func (p *kramosScenario) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		o := ref.E.G.Obj(e.Obj)
		if e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount == -5 && o != nil && o.Face() != nil && o.Face().Name == "Ramos, Dragon Engine" {
			p.tl.removed++
		}
	}
	p.evAt = len(evs)
	return nil
}
