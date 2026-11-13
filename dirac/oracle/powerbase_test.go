package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Kutzil, Malamet Exemplar's
// DamageDoneOnce line reads ValidSource$ Creature.YouCtrl+powerGTbasePower,
// the current layered power against the base through layer 7b (CR 613.4;
// kernel powerGTBase). The G/W decks raise power over base with a 7c pump
// (Giant Growth), a 7c static (Glorious Anthem) and 7d +1/+1 counters
// (Travel Preparations, Battlegrowth) beside unmodified bodies, so some
// connects draw and some do not.
//
// The tally reads gorge's log per combat damage step (StepChange into
// StepCombatDamage up to the next StepChange): a positive Damage event to a
// player while the active player controls a Kutzil is a connect; a Kutzil
// TriggerPush and a Draw by the active player inside the step say whether
// the line fired and resolved.
var kpowerbaseDecks = [2][]cardset.Entry{
	{
		{Name: "Kutzil, Malamet Exemplar", Count: 4}, {Name: "Giant Growth", Count: 3}, {Name: "Glorious Anthem", Count: 2},
		{Name: "Travel Preparations", Count: 3}, {Name: "Battlegrowth", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Kutzil, Malamet Exemplar", Count: 4}, {Name: "Giant Growth", Count: 3}, {Name: "Glorious Anthem", Count: 2},
		{Name: "Travel Preparations", Count: 3}, {Name: "Battlegrowth", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 8},
	},
}

type kpowerbaseTally struct {
	evAt                    int
	inStep                  bool
	hit, pushed, drew       bool
	connects, draws, noDraw int
	pushes                  int
}

func (c *kpowerbaseTally) flush() {
	if c.inStep && c.hit {
		c.connects++
		if c.drew {
			c.draws++
		} else {
			c.noDraw++
		}
		if c.pushed {
			c.pushes++
		}
	}
	c.inStep, c.hit, c.pushed, c.drew = false, false, false, false
}

func kpowerbaseKutzil(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().Name == "Kutzil, Malamet Exemplar" {
			return true
		}
	}
	return false
}

func (c *kpowerbaseTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
		c.inStep, c.hit, c.pushed, c.drew = false, false, false, false
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.StepChange, events.TurnChange:
			c.flush()
			c.inStep = e.Kind == events.StepChange && e.Step == state.StepCombatDamage
		case events.Damage:
			if c.inStep && e.Obj == 0 && e.Amount > 0 && e.Player != g.Active && kpowerbaseKutzil(g, g.Active) {
				c.hit = true
			}
		case events.TriggerPush:
			if o := g.Obj(e.Obj); c.inStep && o != nil && o.Face() != nil && o.Face().Name == "Kutzil, Malamet Exemplar" {
				c.pushed = true
			}
		case events.Draw:
			if c.inStep && c.hit && e.Player == g.Active {
				c.drew = true
			}
		}
	}
	c.evAt = len(evs)
	return nil
}

// kpowerbaseRun plays seeds 1..seeds; stopOnDiv returns at the first
// divergence (the mutation only needs existence).
func kpowerbaseRun(t *testing.T, seeds uint64, mutate bool) (div bool, stalls int, tally *kpowerbaseTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("powerbase")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kpowerbaseDecks)
	tbl, _, _, err := set.Matchup(kpowerbaseDecks[0], kpowerbaseDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally = &kpowerbaseTally{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpowerbase-a", "kpowerbase-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.evAt = 0
		res := Lockstep(m, seed, k, katkamountPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check})
		CloseSide(k)
		tally.flush()
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div = true
			if mutate {
				t.Logf("mutation seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestPowerbaseLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, c := kpowerbaseRun(t, 300, false)
	t.Logf("diverged %v, stalls %d; Kutzil-controller connects %d: drew %d, no draw %d (Kutzil pushes %d)",
		div, stalls, c.connects, c.draws, c.noDraw, c.pushes)
	if div {
		return
	}
	if c.connects < 20 {
		t.Errorf("Kutzil-controller combat-damage steps reached %d, want at least 20", c.connects)
	}
	if c.draws < 10 {
		t.Errorf("connects that drew reached %d, want at least 10", c.draws)
	}
	if c.noDraw < 10 {
		t.Errorf("connects that did not draw reached %d, want at least 10", c.noDraw)
	}
}

// TestPowerbaseMutation: with >= instead of >, an unmodified creature
// matches powerGTbasePower and Kutzil draws on every connect, so the
// lockstep must diverge.
func TestPowerbaseMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPowerGTBaseGE = true
	defer func() { dirac.DebugPowerGTBaseGE = false }()
	div, _, c := kpowerbaseRun(t, 128, true)
	if !div {
		t.Fatalf(">= mutation: no divergence in 128 seeds (connects %d, drew %d, no draw %d)", c.connects, c.draws, c.noDraw)
	}
}
