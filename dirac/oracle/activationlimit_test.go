package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dragon Whelp's ConditionActivationLimit$ GE4 (CR 602.2a, 608.2): "if this
// ability has been activated four or more times this turn, sacrifice it at
// the beginning of the next end step". gorge counts the activation that is
// resolving, and every activation still on the stack, so four stacked
// activations all see four; the kernel reads the same figure from the
// object's activation census (kop.ExActivations).

// activationLimitDeck is mono-red: Dragon Whelps beside Lightning Bolts (a
// Whelp that dies in response to its own activations) and Mountains.
func activationLimitDeck() []string {
	var d []string
	for range 8 {
		d = append(d, "Dragon Whelp")
	}
	for range 4 {
		d = append(d, "Lightning Bolt")
	}
	for len(d) < 40 {
		d = append(d, "Mountain")
	}
	return d
}

// activationLimitObs is what the games reached: the activation picks of a
// Whelp, and the most any one Whelp was picked in one turn.
type activationLimitObs struct {
	picks   int
	maxTurn int
	perTurn map[[2]int32]int // {turn, Whelp's instance} -> activations picked
	armed   int              // delayed sacrifices gorge registered after four or more activations
	early   int              // delayed sacrifices gorge registered with fewer than four pushed (CR 602.2a, 608.2)
}

// tallyArmed reads gorge's log: each DelayedRegister of a Whelp's resolving
// ability counts the AbilityPush events of that Whelp this turn so far.
func (o *activationLimitObs) tallyArmed(g *GorgeSide) {
	pushes := map[state.ObjID]int{}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			clear(pushes)
		case events.AbilityPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Dragon Whelp" {
				pushes[ev.Obj]++
			}
		case events.DelayedRegister:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Dragon Whelp" {
				if pushes[ev.Obj] >= 4 {
					o.armed++
				} else {
					o.early++
				}
			}
		}
	}
}

// activationLimitPolicy activates a Whelp whenever it can (so the activations
// stack up before anyone passes), else casts and plays lands, else random.
func activationLimitPolicy(m *Matchup, seed uint64, obs *activationLimitObs) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || seed%5 == 0 {
			return rnd.Pick(c)
		}
		var act, cast, land []Canon
		for _, o := range c.D.Options {
			switch {
			case o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Dragon Whelp":
				act = append(act, o)
			case o.Kind() == ActCast:
				cast = append(cast, o)
			case o.Kind() == ActPlayLand:
				land = append(land, o)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(act) > 0 && c.G != nil:
			a := pick(act)
			k := [2]int32{c.G.E.G.Turn, int32(a.Src())}
			obs.perTurn[k]++
			obs.picks++
			obs.maxTurn = max(obs.maxTurn, obs.perTurn[k])
			return a
		case len(land) > 0:
			return pick(land)
		case len(cast) > 0:
			return pick(cast)
		}
		return rnd.Pick(c)
	})
}

// runActivationLimit plays the matchup; mutate makes the kernel read no
// activations (the mutation must diverge).
func runActivationLimit(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *activationLimitObs) {
	t.Helper()
	reg := synthReg(t)
	deck := activationLimitDeck()
	set := cardset.AllowUnverified(printedDelveSet(t, [2][]string{deck, deck}))
	old := dirac.DebugNoActCount
	dirac.DebugNoActCount = mutate
	defer func() { dirac.DebugNoActCount = old }()
	obs = &activationLimitObs{perTurn: map[[2]int32]int{}}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range deck {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(deck), entries(deck))
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"actlimit-a", "actlimit-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, activationLimitPolicy(m, seed, obs), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20})
		obs.tallyArmed(g)
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, obs
}

// TestActivationLimitLockstep plays Dragon Whelp games: four or more
// activations in a turn (stacked and resolved one by one) sacrifice it at the
// next end step in both engines, and the games reach that many activations.
func TestActivationLimitLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, obs := runActivationLimit(t, 40, false)
	t.Logf("%d games, %d divergences, %d stalls, %d Whelp activations, at most %d by one Whelp in a turn, %d sacrifices armed (%d early)", games, divs, stalls, obs.picks, obs.maxTurn, obs.armed, obs.early)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.early != 0 {
		t.Errorf("gorge armed %d Whelp sacrifices with fewer than four activations pushed", obs.early)
	}
	if obs.armed == 0 {
		t.Errorf("gorge armed no Whelp sacrifice after four activations")
	}
	if obs.maxTurn < 4 {
		t.Fatalf("no Whelp was activated four times in a turn (most %d): the gate was never reached", obs.maxTurn)
	}
}

// TestActivationLimitMutation makes the kernel read no activations: it
// never sacrifices a Whelp activated four times, which gorge does.
func TestActivationLimitMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runActivationLimit(t, 40, true)
	if divs == 0 {
		t.Fatalf("mutation read no activations in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
