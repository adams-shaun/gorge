package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The '!' negations of filter words: Prophesied End's
// Card.!attacking on the destroyed target, Moment of Glory's Creature.YouCtrl+!targetedBy,
// Cactuar's Card.Self+!ThisTurnEntered, Roiling Canopy's
// Forest.YouCtrl+!TriggeredCard and Spider-Man's
// Creature.Other+!attacking+YouCtrl. The kernel's PredNotF2 and
// FNotAttacking reads must agree with gorge's matchPredicate negation at
// every decision.

var kpropnegDecks = [][2][]cardset.Entry{{
	// A: green/white, the attackers and the instants
	{
		{Name: "Prophesied End", Count: 4}, {Name: "Spider-Man, To the Rescue", Count: 4}, {Name: "Roiling Canopy", Count: 4},
		{Name: "Moment of Glory", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Cactuar", Count: 3},
		{Name: "Forest", Count: 11}, {Name: "Plains", Count: 7},
	},
	// B: black/red, the burn and the blockers
	{
		{Name: "Scathe Zombies", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Mountain", Count: 10},
		{Name: "Swamp", Count: 10},
	},
}, {
	// B against A, the seats swapped
	{
		{Name: "Scathe Zombies", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Mountain", Count: 10},
		{Name: "Swamp", Count: 10},
	},
	{
		{Name: "Prophesied End", Count: 4}, {Name: "Spider-Man, To the Rescue", Count: 4}, {Name: "Roiling Canopy", Count: 4},
		{Name: "Moment of Glory", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Cactuar", Count: 3},
		{Name: "Forest", Count: 11}, {Name: "Plains", Count: 7},
	},
}}

// kpropnegStats is what the policy counts across the games.
type kpropnegStats struct {
	peRes, peDraw, peNoDraw        int // Prophesied End resolutions; those that drew; those that killed without drawing
	mogRes, mogFlash               int // Moment of Glory resolutions; those that put a counter on another creature
	cactEnter, cactRet, cactStay   int // Cactuar entries, end-step returns, end steps it stayed (entered that turn)
	spiderEtb, canopyEtb, canopyGo int // Spider-Man triggers; Forest entries; Canopy triggers (six Forests out)
	peCombat                       int // Prophesied End resolutions during the declare-blockers .. end-of-combat steps
	peAtk, peAtkDraw               int // Prophesied End kills of a creature that was attacking; those that drew anyway
}

// kpropnegTally scans the gorge event log for the carrier events.
type kpropnegTally struct {
	evAt   int
	turn   int
	active state.PlayerID
	on     string // the carrier whose resolution is being read
	ctrs   int    // counters put on inside the window
	drew   bool
	killed bool
	step   state.Step
	atk    map[state.ObjID]bool // this turn's declared attackers
	atkHit bool                 // the window's victim was attacking
	ent    map[state.ObjID]int  // Cactuar -> the turn it entered
}

func (p *kpropnegTally) close(st *kpropnegStats) {
	switch p.on {
	case "Prophesied End":
		if p.atkHit {
			st.peAtk++
			if p.drew {
				st.peAtkDraw++
			}
		}
		if p.drew {
			st.peDraw++
		} else if p.killed {
			st.peNoDraw++
		}
	case "Moment of Glory":
		if p.ctrs > 1 {
			st.mogFlash++
		}
	}
	p.on, p.ctrs, p.drew, p.killed, p.atkHit = "", 0, false, false, false
}

func (p *kpropnegTally) scan(ref *GorgeSide, st *kpropnegStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			p.turn++
			p.active = e.Player
			clear(p.atk)
		case events.DeclareAttackers:
			if p.atk == nil {
				p.atk = map[state.ObjID]bool{}
			}
			for _, id := range e.IDs {
				p.atk[id] = true
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Spider-Man, To the Rescue":
				st.spiderEtb++
			case "Cactuar":
				st.cactRet++
			case "Roiling Canopy":
				st.canopyGo++
			}
		case events.Resolve:
			p.close(st)
			switch n := chName(g, e.Obj); n {
			case "Prophesied End":
				st.peRes++
				if p.step >= state.StepDeclareBlockers && p.step <= state.StepEndCombat {
					st.peCombat++
				}
				p.on = n
			case "Moment of Glory":
				st.mogRes++
				p.on = n
			}
		case events.Priority:
			p.close(st)
		case events.Draw:
			if p.on != "" {
				p.drew = true
			}
		case events.CounterChange:
			if p.on == "Moment of Glory" && e.Amount > 0 {
				p.ctrs++
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard && p.on != "" {
				p.killed = true
				p.atkHit = p.atkHit || p.atk[e.Obj]
			}
			if e.To == state.ZBattlefield {
				switch chName(g, e.Obj) {
				case "Cactuar":
					st.cactEnter++
					if p.ent == nil {
						p.ent = map[state.ObjID]int{}
					}
					p.ent[e.Obj] = p.turn
				case "Forest":
					st.canopyEtb++
				}
			}
		case events.StepChange:
			p.step = e.Step
			if e.Step == state.StepEnd {
				for id, t := range p.ent {
					if ob := g.Obj(id); ob != nil && ob.Zone == state.ZBattlefield && ob.Controller == p.active && t == p.turn {
						st.cactStay++
						delete(p.ent, id)
					}
				}
			}
		}
	}
	p.evAt = len(evs)
}

// kpropnegPolicy prefers casting or activating (so the carriers are reached),
// else random, and counts the carrier events the gorge side logs.
func kpropnegPolicy(seed uint64, st *kpropnegStats) Policy {
	rnd := RandomPolicy(seed)
	var buf, all []Canon
	var tl kpropnegTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		// Prophesied End is held for the combat steps, so it meets
		// attacking and non-attacking targets
		all, buf = all[:0], buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && c.G != nil && chName(c.G.E.G, state.ObjID(a.Src())+1) == "Prophesied End" &&
				(c.G.E.G.Step < state.StepDeclareBlockers || c.G.E.G.Step > state.StepEndCombat) {
				continue
			}
			all = append(all, a)
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			buf = all
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runPropneg plays the constructed matchups and returns the games, the
// divergences, the stalls and the carrier counts. mutate makes the kernel's
// PredNotF2 answer the positive flag (the mutation must diverge).
func runPropneg(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kpropnegStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("propneg")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPropNegPositive
	dirac.DebugPropNegPositive = mutate
	defer func() { dirac.DebugPropNegPositive = old }()
	for mi, md := range kpropnegDecks {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kpropneg-a", "kpropneg-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, kpropnegPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestPropnegLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runPropneg(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	// Both sides draw for a destroyed ATTACKER too (peAtk == peAtkDraw): gorge
	// reads !attacking on the graveyard object, so Prophesied End never
	// reaches its no-draw branch (see the report's gorge finding). The test
	// pins the agreement and that attacking targets were reached.
	if st.peRes < 10 || st.peDraw < 1 || st.peAtk < 1 {
		t.Errorf("Prophesied End: %d resolutions, %d draws, %d attacking victims (want >= 10, >= 1, >= 1)", st.peRes, st.peDraw, st.peAtk)
	}
	if st.mogRes < 5 {
		t.Errorf("only %d Moment of Glory resolutions (want >= 5)", st.mogRes)
	}
	if st.cactRet < 10 || st.cactStay < 5 {
		t.Errorf("Cactuar: %d end-step returns, %d stays (want >= 10, >= 5)", st.cactRet, st.cactStay)
	}
}

// TestPropnegMutation makes PredNotF2 answer the positive flag: gorge's '!'
// negation is the opposite, so the games must diverge. A green mutation
// means the lockstep never reaches a decision the negation shapes.
func TestPropnegMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runPropneg(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation answered positive in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
