package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestKillerAmongUsLowers plays A Killer Among Us in lockstep with gorge
// over the killeramongus fixture pool (dirac/cardset/pools/fixtures/
// killeramongus): its enters trigger makes a Human, a Merfolk and a Goblin
// token and then secretly chooses one of those three types (gorge
// effChooseType over ValidTypes$, the answer marked Secret; kgen
// chooseCreatureTypeMid, the kernel's midTypeOptions), and its "Sacrifice,
// Reveal the chosen creature type" ability puts three +1/+1 counters and
// deathtouch on the targeted attacking token only if that token is the
// chosen type (Defined$ Targeted.ChosenType beside its own ValidTgts$, kgen
// ownTargetedGuard; the RevealChosen<Type> cost is the offer gate
// revealChosenGate).
//
// Precondition: the trigger's type ask is posed with exactly the three
// valid types, the ability is activated, and both a chosen-type target (the
// counters land) and an other-type target (nothing happens) occur in
// gorge's logs. The mutations, each of which must diverge: the guard
// dropped (the counters land whatever the token's type), and the ask
// offering the whole creature-type vocabulary.
func TestKillerAmongUsLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runKillerLockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, %+v", run.games, run.divs, run.stalls, st)
		if st.asks == 0 || st.badAsk != 0 || st.secret == 0 {
			t.Errorf("the secret type ask: %d posed, %d not over exactly Human, Merfolk, Goblin, %d answers marked Secret", st.asks, st.badAsk, st.secret)
		}
		if st.hits == 0 || st.acts <= st.hits {
			t.Errorf("activations %d, chosen-type hits %d: want both a hit and a miss", st.acts, st.hits)
		}
	})
	t.Run("mutation-guard", func(t *testing.T) {
		run, _ := runKillerLockstep(t, 24, false, func(tbl *kop.Table) {
			mutateCard(t, tbl, "A Killer Among Us", func(a *kop.Ability, in *kop.Instr) {
				if a.Kind == kop.AbActivated && in.Op == kop.OpSkipUnless {
					in.Op = kop.OpNop // the chosen type ignored: every target gets the counters
				}
			})
		})
		run.requireDivergence(t, "Targeted.ChosenType's guard dropped")
	})
	t.Run("mutation-validtypes", func(t *testing.T) {
		dirac.DebugChooseTypeAllVocab = true
		defer func() { dirac.DebugChooseTypeAllVocab = false }()
		run, _ := runKillerLockstep(t, 8, false, nil)
		run.requireDivergence(t, "the secret type ask over the whole vocabulary")
	})
}

var killerDecks = fixtureDecks{
	{{Name: "A Killer Among Us", Count: 12}, {Name: "Llanowar Elves", Count: 6}, {Name: "Forest", Count: 22}},
	{{Name: "A Killer Among Us", Count: 12}, {Name: "Llanowar Elves", Count: 6}, {Name: "Forest", Count: 22}},
}

// killerStats is what gorge's logs show of the watched card.
type killerStats struct {
	asks, badAsk int // the trigger's type asks, and those not over exactly the valid three
	acts, hits   int // the ability's activations, and its +1/+1 placements
	secret       int // gorge's type answers marked Secret (Secretly$ True)
}

// killerPolicy plays randomly but casts A Killer Among Us, attacks with
// every creature and activates the ability whenever each is offered (seven
// times in eight).
func killerPolicy(seed uint64, st *killerStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b696c6c)) // "kill"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Kind == DAttackers && prng.IntN(8) != 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					return a
				}
			}
		}
		if c.D.Kind == DPriority && prng.IntN(8) != 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); (k == ActActivate || k == ActCast) && refCardName(c.G, a.Src()) == "A Killer Among Us" {
					return a
				}
			}
		}
		if killerTypeAsk(c) {
			st.asks++
			if !killerAskValid(c) {
				st.badAsk++
			}
		}
		return rnd.Pick(c)
	})
}

// killerTypeAsk reports whether c is gorge's creature-type ask: every option
// of the pending decision a creature-type label.
func killerTypeAsk(c *PickCtx) bool {
	d := c.G.Pending()
	if d == nil || len(d.Options) == 0 {
		return false
	}
	for _, o := range d.Options {
		if !effects.CreatureTypeWords(o.Label) {
			return false
		}
	}
	return true
}

var killerValid = map[string]bool{"Human": true, "Merfolk": true, "Goblin": true}

// killerAskValid reports whether the pending type ask offers exactly the
// three valid types.
func killerAskValid(c *PickCtx) bool {
	opts := c.G.Pending().Options
	if len(opts) != len(killerValid) {
		return false
	}
	for _, o := range opts {
		if !killerValid[o.Label] {
			return false
		}
	}
	return true
}

// runKillerLockstep plays seeds 1..seeds of the killeramongus matchup,
// gorge leading; mutate rewrites the table first.
func runKillerLockstep(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, killerStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("killeramongus")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range killerDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(killerDecks[0], killerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	killerTableLowered(t, tbl)
	if mutate != nil {
		mutate(tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	var st killerStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"killer-a", "killer-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, killerPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		killerLogStats(g, &st)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}

// killerTableLowered fails unless the table carries the three lowered
// pieces: the trigger's narrowed ChooseType, the activation condition and
// the ability's guard.
func killerTableLowered(t *testing.T, tbl *kop.Table) {
	t.Helper()
	var ask, gate, guard bool
	for _, cd := range tbl.Cards {
		if cd.Name != "A Killer Among Us" {
			continue
		}
		ask = len(cd.TypeValid) == 3
		for _, a := range cd.Abilities {
			for _, in := range a.Prog {
				ask = ask && !(a.Kind == kop.AbTriggered && in.Op == kop.OpChooseType && in.B.N == 0)
				guard = guard || a.Kind == kop.AbActivated && in.Op == kop.OpSkipUnless
			}
			gate = gate || a.Kind == kop.AbActivated && a.Trig.Cond != 0
		}
	}
	if !ask || !gate || !guard {
		t.Fatalf("A Killer Among Us: narrowed ask %v, RevealChosen gate %v, Targeted.ChosenType guard %v (not lowered)", ask, gate, guard)
	}
}

// killerLogStats adds the activations of A Killer Among Us (gorge's
// AbilityPush events), its secret type answers and the +1/+1 counters
// placed (nothing else in the pool places one) in g's log.
func killerLogStats(g *GorgeSide, st *killerStats) {
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.AbilityPush:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) != 0 && o.Card.Faces[0].Name == "A Killer Among Us" {
				st.acts++
			}
		case events.Choose:
			if e.Counter == "type" && e.Secret {
				st.secret++
			}
		case events.CounterChange:
			if strings.EqualFold(e.Counter, "P1P1") && e.Amount > 0 {
				st.hits++
			}
		}
	}
}
