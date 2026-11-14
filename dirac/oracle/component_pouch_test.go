package oracle

import (
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The dead mana-ability census gap (Component Pouch): "{T}, Remove a
// component counter from Component Pouch: Add two mana of different colors."
// gorge's planner records the source as an unrelaxable, unbounded census gap
// while the cost is payable (potential_plan.go paymentPlanCensusOf;
// paymentPlanRelaxedAlternatives enumerates no colour for the Produced$ Combo
// AnyDifferent production), and its resolution fail-closes with no mana
// (engine/effects/mana_effect.go's rune gate). The kernel models it as a dead
// chain member of the prefix search (dirac mana_countercost.go ctrDeadGap):
// never a planner unit or window pick, no units in any gaps walk, its one
// chain branch pays the cost and taps without mana, and the search's slot
// accounting counts every insufficient play whatever it finds (potentialPlans
// counts one before the search runs).

// The d20 (its {T}: Roll a d20 ability) puts the component counters on; the
// Wall of Roots beside it is the real counter-cost census gap the chain's
// later step activates. Fyndhorn Elves and the four creatures are the plays
// whose costs reach past the lands, so the counter-cost gaps fund them.
var kpouchDecks = [2][]cardset.Entry{
	{
		{Name: "Component Pouch", Count: 8}, {Name: "Wall of Roots", Count: 4},
		{Name: "Leatherback Baloth", Count: 4}, {Name: "Strangleroot Geist", Count: 4},
		{Name: "Experiment One", Count: 4}, {Name: "Young Wolf", Count: 4},
		{Name: "Fyndhorn Elves", Count: 4}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Syndic of Tithes", Count: 4}, {Name: "Spirited Companion", Count: 4},
		{Name: "Sentinel of the Eternal Watch", Count: 4}, {Name: "Recumbent Bliss", Count: 4},
		{Name: "Grimclaw Bats", Count: 4}, {Name: "Preordain", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
	},
}

// TestComponentPouchLowers plays Component Pouch in lockstep with gorge
// leading: the deck casts it, rolls its d20 for counters, and spends the
// counters through the dead chain steps a scripted play activates. Each
// carrier must be played by some game.
func TestComponentPouchLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	carriers := []string{"Component Pouch", "Wall of Roots"}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Set(t, "mbc")
	var mu sync.Mutex
	played := map[string]int{}
	decks := kpouchDecks
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		var cs [2][]*cards.Card
		for s, d := range decks {
			for _, en := range d {
				card, ok := reg.Lookup(en.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", en.Name)
				}
				for i := 0; i < en.Count; i++ {
					cs[s] = append(cs[s], card)
				}
			}
		}
		m, err := NewMatchup(reg, [2]string{"componentpouch-a", "componentpouch-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		for _, name := range res.RecentSources(m, len(res.Actions)) {
			if slices.Contains(carriers, name) {
				mu.Lock()
				played[name]++
				mu.Unlock()
			}
		}
	}
	for _, name := range carriers {
		if played[name] == 0 {
			t.Errorf("%s was never played in 60 games: the pool does not reach it", name)
		}
	}
	t.Logf("componentpouch: carriers played %v", played)
}

// TestDeadGapScriptChain is the chain itself, in one deterministic game:
// gorge's prefix script activates the dead pouch (Component Pouch, one
// component counter on it, untapped) before the real gap (a Wall of Roots
// that entered after it, so the census's gap order is [dead, roots]) and both
// sides replay the same steps. The board: one Forest is the only green mana
// the lands make, so Colossal Dreadmaw ({4}{G}{G}) is payable only through the
// roots gap -- the planner is insufficient, the census is incomplete and, the
// dead gap being unrelaxable, the prefix search explores the gaps in
// battlefield order and finds [dead, roots] before [roots]. The policy casts
// the pouch before the roots so the order holds, rolls the d20 for the
// counter, and casts the Dreadmaw when it is offered.
//
// The mutation round proves the pool reaches it: with the dead gaps dropped
// from the kernel's chain the canonical script steps differ (gorge's [dead,
// roots] against the kernel's [roots]) and some seed diverges -- the dead
// step's cost is gorge's alone, so the pouch stays tapped with its counter
// spent on the kernel's side.
func TestDeadGapScriptChain(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		t.Parallel()
		tal := kpouchChainRun(t, 24, true)
		if tal.dead == 0 {
			t.Fatal("the dead pouch activation never ran: the script never activates Component Pouch before the roots")
		}
	})
	t.Run("mutation", func(t *testing.T) {
		dirac.DebugNoDeadCtr = true
		defer func() { dirac.DebugNoDeadCtr = false }()
		tal := kpouchChainRun(t, 40, false)
		if tal.divs == 0 {
			t.Fatal("no seed diverged without the dead gaps: the game never funds a play through the [dead pouch, roots] script")
		}
	})
}

// kpouchChainDecks: player 0 holds one Forest, so no land pair makes the two
// {G} a Colossal Dreadmaw needs; the Wall of Roots is the second {G}. The
// Plains pay the Dreadmaw's {4} and the Pouch's {3}; player 1 has only
// Islands and does nothing.
var kpouchChainDecks = [2][]cardset.Entry{
	{
		{Name: "Component Pouch", Count: 2}, {Name: "Wall of Roots", Count: 2},
		{Name: "Colossal Dreadmaw", Count: 4},
		{Name: "Plains", Count: 31}, {Name: "Forest", Count: 1},
	},
	{{Name: "Island", Count: 40}},
}

// kpouchTally counts, on gorge's side, the dead activations (the cost's
// component counter removed from the pouch) and the games that diverged.
type kpouchTally struct {
	games, divs, dead int
}

// kpouchChainRun plays the chain scenario seeds 1..n in lockstep with gorge
// leading, failing on each divergence when report; it returns the tally.
func kpouchChainRun(t *testing.T, seeds uint64, report bool) kpouchTally {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Set(t, "mbc")
	decks, tbl := mccoyBoard(t, reg, set, kpouchChainDecks)
	var tl kpouchTally
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"pouchchain-a", "pouchchain-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &kpouchChainScenario{}
		res := Lockstep(m, seed, k, sc, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: sc.check})
		CloseSide(k)
		tl.games++
		tl.dead += sc.dead
		if res.Div != nil {
			tl.divs++
			if !report {
				t.Logf("seed %d diverges at decision %d: %s", seed, res.Div.At, res.Div.What)
				return tl
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
				res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.Stall != "" && res.Stall != "turns" {
			t.Fatalf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return tl
}

// kpouchChainScenario drives player 0 to the chain: a land, the pouch, the
// roots behind it, the roll, then the Dreadmaws the [dead, roots] script
// pays. Player 1 passes.
type kpouchChainScenario struct {
	// dead is the cost's component counter removed from a Component Pouch,
	// read off gorge's events by check. roll is the +1 that puts one on; dmaw
	// a Dreadmaw cast.
	dead, roll, dmaw int
	ref              *GorgeSide
	evAt             int
}

func (p *kpouchChainScenario) Pick(c *PickCtx) Canon {
	return mulliganDonePick(p.priority, c)
}

func (p *kpouchChainScenario) priority(c *PickCtx) Canon {
	g := c.G.E.G
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	// the Dreadmaw is offered exactly when the [dead, roots] script (or the
	// lands alone) pays it; pick it before anything taps the pouch again
	if a, ok := reproHandCastAction(c, g, 0, "Colossal Dreadmaw"); ok {
		return a
	}
	if a, ok := p.firstHandCast(c, g, "Component Pouch"); ok {
		return a
	}
	// the roots only once the pouch is on the battlefield (or none is left in
	// hand): the census's gap order is the battlefield order, so the dead
	// source must enter first
	if reproBattlefieldObj(g, "Component Pouch", 0) == nil && kramosInHand(g, "Component Pouch") {
		return pickKind(c, ActPass)
	}
	if a, ok := p.firstHandCast(c, g, "Wall of Roots"); ok {
		return a
	}
	if pouch := reproBattlefieldObj(g, "Component Pouch", 0); pouch != nil && g.Step == state.StepMain2 {
		ref := c.G.InstRef(pouch.ID)
		if a, ok := findAction(c, func(a Canon) bool {
			return a.Kind() == ActActivate && a.Src() == ref && a.Mode() == 1 // the d20 roll
		}); ok {
			return a
		}
	}
	return pickKind(c, ActPass)
}

// firstHandCast returns the cast action for the first named card held whose
// cast is offered and not already on the battlefield (the legend rule).
func (p *kpouchChainScenario) firstHandCast(c *PickCtx, g *state.Game, names ...string) (Canon, bool) {
	for _, name := range names {
		if controlsBattlefieldNamed(g, 0, name) {
			continue
		}
		if a, ok := reproHandCastAction(c, g, 0, name); ok {
			return a, true
		}
	}
	return 0, false
}

// check counts gorge's component counter removed from a Component Pouch: the
// dead activation's cost, paid in the script's first step.
func (p *kpouchChainScenario) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		o := ref.E.G.Obj(e.Obj)
		n := ""
		if o != nil && o.Face() != nil {
			n = o.Face().Name
		}
		switch {
		case e.Kind == events.CounterChange && e.Counter == "COMPONENT" && e.Amount == -1 && n == "Component Pouch":
			p.dead++
		case e.Kind == events.CounterChange && e.Counter == "COMPONENT" && e.Amount > 0 && n == "Component Pouch":
			p.roll += int(e.Amount)
		case e.Kind == events.PutOnStack && n == "Colossal Dreadmaw":
			p.dmaw++
		}
	}
	p.evAt = len(evs)
	return nil
}
