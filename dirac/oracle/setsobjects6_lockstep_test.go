package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The setsobjects6 fixture pool's lockstep runs: ExtraPhase$ Beginning (kgen
// lower_addphase.go addPhaseBeginning, the kernel's act_addphase.go
// kop.AddPhaseBeginning). Sphinx of the Second Sun and Shadow of the Second
// Sun add a beginning phase (untap, upkeep, draw) after the postcombat main
// phase; the walk then takes the end step. The scenario is reached when the
// gorge side enters an extra beginning phase's draw step. Two mutations must
// diverge: the phase ends after its upkeep (no draw), and its untap step ends
// summoning sickness as a new turn's would (Prodigal Sorcerer and Llanowar
// Elves cast this turn tap in the extra upkeep).
//
// The expected answer at every decision is gorge's.

func so6Deck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Sphinx of the Second Sun", Count: 4}, {Name: "Shadow of the Second Sun", Count: 4},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Opt", Count: 2}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	}
}

// so6Seen counts the distinct turns whose extra beginning phase reached its
// draw step, on gorge's state.
type so6Seen struct {
	turns map[int32]bool
}

func (s *so6Seen) observe(g *state.Game) {
	if g.Step != state.StepDraw {
		return
	}
	for _, ep := range g.ExtraPhases {
		if ep.Consumed && ep.Entry == state.StepUntap {
			s.turns[int32(g.Turn)] = true
		}
	}
}

// runSO6 plays seeds games of the deck at every seat; mutated runs may
// diverge or stall.
func runSO6(t *testing.T, seats int, lo, hi uint64, mutated bool) (fixtureRun, int) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("setsobjects6")
	if err != nil {
		t.Fatal(err)
	}
	list := so6Deck()
	var deck []string
	for _, e := range list {
		for range e.Count {
			deck = append(deck, e.Name)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	lists := make([][]cardset.Entry, seats)
	for i := range lists {
		lists[i] = list
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	seen := &so6Seen{turns: map[int32]bool{}}
	extra := 0
	var run fixtureRun
	for seed := lo; seed <= hi; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"so6-0", "so6-1"}, Cards: [2][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		for s := 2; s < seats; s++ {
			r.MoreDecks = append(r.MoreDecks, fmt.Sprintf("so6-%d", s))
			r.MoreCards = append(r.MoreCards, deck)
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 2500*seats, int32(13*seats)
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		clear(seen.turns)
		base := castFirstPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				seen.observe(c.G.E.G)
			}
			return base.Pick(c)
		})
		res := LockstepLed(m, seed, g, k, pol, rc)
		CloseSide(k)
		extra += len(seen.turns)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.StallUnmodelled {
			if !mutated {
				t.Fatalf("%d seats, seed %d: kernel stopped: %s", seats, seed, res.Stall)
			}
			run.stalls++
		}
		if d := res.Div; d != nil {
			if !mutated {
				t.Fatalf("%d seats, seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seats, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			run.divs++
			if run.first == "" {
				run.first = fmt.Sprintf("seed %d at %d: %s %s", seed, d.At, d.What, d.Field)
			}
		}
	}
	return run, extra
}

func TestSetsObjects6BeginningLockstep(t *testing.T) {
	for _, seats := range []int{2, 4} {
		t.Run(fmt.Sprintf("seats%d", seats), func(t *testing.T) {
			run, extra := runSO6(t, seats, 1, 16, false)
			t.Logf("%d seats, %d games: %d extra beginning phases reached their draw step", seats, run.games, extra)
			if extra == 0 {
				t.Errorf("%d seats: no extra beginning phase reached its draw step", seats)
			}
		})
	}
}

func TestSetsObjects6BeginningMutation(t *testing.T) {
	t.Run("mutation-no-draw-step", func(t *testing.T) {
		dirac.DebugAddPhaseBeginningShort = true
		defer func() { dirac.DebugAddPhaseBeginningShort = false }()
		run, _ := runSO6(t, 2, 1, 16, true)
		run.requireDivergence(t, "the extra beginning phase ended after its upkeep")
	})
	t.Run("mutation-untap-clears-sickness", func(t *testing.T) {
		dirac.DebugAddPhaseUntapClearsSick = true
		defer func() { dirac.DebugAddPhaseUntapClearsSick = false }()
		run, _ := runSO6(t, 2, 1, 16, true)
		run.requireDivergence(t, "the extra untap step ended summoning sickness")
	})
}
