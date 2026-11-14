package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCCosts9Lowers plays the MBC costs-9 spellings kgen lowers, in
// lockstep with gorge over the mbccosts9 fixture pool
// (dirac/cardset/pools/fixtures/mbccosts9):
//
//   - morph cost "PayLife<N>": Zombie Cutthroat's "Morph--Pay 5 life"
//     (CardDef.MorphLife, dirac/face_morphlife.go). The policy casts it face
//     down and turns it up whenever gorge offers the special action, so the
//     life payment settles and gorge withholds the action below 5 life.
//   - resolution mana cost {N(GW)}: Emiel the Blessed's "whenever another
//     creature you control enters, you may pay {G/W}" (a trigger's own
//     hybrid cost, kop.PayManaF, dirac/cost_trighybrid.go). The random
//     policy pays or declines; each payment puts the pool's only +1/+1
//     counters.
//
// Precondition: each watched event happens in gorge's logs (counted below).
// Each mutation subtest drops its lowering from the table and must diverge.
// The other two costs-9 spellings (Thornscape Battlemage's two-part kicker,
// Wash Away's cleave Cost$) stay refused: see the report.
func TestMBCCosts9Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, n := runCosts9Lockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, Zombie Cutthroat turned up %d times, Emiel paid %d times", run.games, run.divs, run.stalls, n.turnUps, n.p1p1)
		if n.turnUps < 5 {
			t.Errorf("Zombie Cutthroat turned face up %d times, want at least 5", n.turnUps)
		}
		if n.p1p1 < 5 {
			t.Errorf("Emiel the Blessed's {G/W} paid %d times (+1/+1 counters), want at least 5", n.p1p1)
		}
	})
	t.Run("MorphLife/mutation", func(t *testing.T) {
		run, _ := runCosts9Lockstep(t, 24, false, func(t *testing.T, tbl *kop.Table) {
			cd := costs9Card(t, tbl, "Zombie Cutthroat")
			if cd.MorphLife != 5 || cd.MorphFamily != kop.MorphMorph {
				t.Fatalf("Zombie Cutthroat: MorphLife %d family %d, want 5 and Morph", cd.MorphLife, cd.MorphFamily)
			}
			cd.MorphLife = 0
		})
		run.requireDivergence(t, "a Morph--Pay 5 life turned up for free")
	})
	t.Run("TrigHybrid/mutation", func(t *testing.T) {
		run, _ := runCosts9Lockstep(t, 24, false, func(t *testing.T, tbl *kop.Table) {
			cd, n := costs9Card(t, tbl, "Emiel the Blessed"), 0
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbTriggered || len(a.Prog) == 0 || !a.Prog[0].IsPayMana() {
					continue
				}
				var c kop.ManaCost
				kop.PayManaHyb(a.Prog[0].F, &c)
				if c.Hyb[0] != kop.ColG|kop.ColW || c.Hyb[1] != 0 {
					t.Fatalf("Emiel the Blessed's trigger cost hybrid %v, want {G/W}", c.Hyb)
				}
				a.Prog[0].F = kop.FilterID(kop.PayManaLife(a.Prog[0].F)) // the {G/W} pip dropped: a free pay
				n++
			}
			if n != 1 {
				t.Fatalf("Emiel the Blessed: %d triggers with a PayMana own cost, want 1", n)
			}
		})
		run.requireDivergence(t, "Emiel the Blessed's {G/W} paid for free")
	})
}

// costs9Card is the table's card named name.
func costs9Card(t *testing.T, tbl *kop.Table, name string) *kop.CardDef {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			return &tbl.Cards[i]
		}
	}
	t.Fatalf("%s not in the table", name)
	return nil
}

// costs9Watch is the watched cards; the policy favours their casts and
// special actions.
var costs9Watch = map[string]bool{"Zombie Cutthroat": true, "Priest of Gix": true, "Emiel the Blessed": true}

var costs9Decks = fixtureDecks{
	{{Name: "Zombie Cutthroat", Count: 12}, {Name: "Priest of Gix", Count: 8}, {Name: "Mox Jet", Count: 4}, {Name: "Swamp", Count: 16}},
	{{Name: "Emiel the Blessed", Count: 8}, {Name: "Grizzly Bears", Count: 14}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 9}},
}

// costs9Counts is what the games did with the watched cards: Zombie
// Cutthroat's turn-ups (the pool's one face-down card) and the +1/+1
// counters put (only Emiel the Blessed's paid trigger puts them).
type costs9Counts struct{ turnUps, p1p1 int }

// costs9Policy plays randomly but, seven times in eight, takes the first
// offered turn-up, face-down cast or cast of a watched card, in that order
// of preference: Priest of Gix floats the {B}{B}{B} a face-down Zombie
// Cutthroat is cast from (gorge prices that cast from the floating pool).
// It never activates Emiel the Blessed's blink over a non-empty stack: a
// creature blinked in response to its own enter trigger returns as a new
// object (CR 400.7), which the kernel's stale trigger no longer counters
// while gorge's TriggeredCardLKICopy still does (a gorge defect:
// effects/counters_put.go putCounterReferentLeft reads no incarnation).
func costs9Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x636f3973)) // "co9s"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		if prng.IntN(8) != 0 {
			for _, k := range [...]CanonKind{ActSpecial, ActCastFace, ActCast} {
				for _, a := range c.D.Options {
					if a.Kind() == k && costs9Watch[refCardName(c.G, a.Src())] {
						return a
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate && len(c.G.E.G.Stack) != 0 && refCardName(c.G, a.Src()) == "Emiel the Blessed" {
				continue
			}
			buf = append(buf, a)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runCosts9Lockstep plays seeds 1..seeds of the mbccosts9 matchup, gorge
// leading, and counts the watched events in gorge's logs. mutate, when set,
// rewrites the table first.
func runCosts9Lockstep(t *testing.T, seeds int, report bool, mutate func(*testing.T, *kop.Table)) (fixtureRun, costs9Counts) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbccosts9")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range costs9Decks {
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
	tbl, _, _, err := set.Matchup(costs9Decks[0], costs9Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(t, tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	var n costs9Counts
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"costs9-a", "costs9-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, costs9Policy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		for _, e := range g.E.L.Events {
			switch {
			case e.Kind == events.TurnFaceUp:
				n.turnUps++
			case e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount > 0:
				n.p1p1++
			}
		}
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
	return run, n
}
