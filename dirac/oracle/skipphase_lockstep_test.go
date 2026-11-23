package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The one-shot DB$ SkipPhase shapes (gorge skipphase_steps.go, dirac
// act_skipphase.go; CR 614.10): the next untap step (Avizoa, Brine Elemental,
// Dovin, Shisato), the next draw step (Fatigue, Ivory Gargoyle, Molten
// Firebird, Elfhame Sanctuary) and the next combat phase, this turn or the
// next one (Moment of Silence, Blinding Angel, Revenant Patriarch, Stonehorn
// Dignitary), for the caster, a target, a triggered player and each opponent.
// Four decks play four seats and the first two play two; the lockstep compares
// every option list, so a step the kernel skips and gorge does not (or the
// reverse) shows as a divergence at the next decision.

func skipShapeDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{ // W/U: the targeted skips and the untap pump
			{Name: "Moment of Silence", Count: 4}, {Name: "Fatigue", Count: 4}, {Name: "Avizoa", Count: 3}, {Name: "Stonehorn Dignitary", Count: 3},
			{Name: "Blinding Angel", Count: 2}, {Name: "Dovin, Architect of Law", Count: 2}, {Name: "Isamaru, Hound of Konda", Count: 2}, {Name: "Savannah Lions", Count: 2},
			{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
		},
		{ // W/B: the triggered-player skips
			{Name: "Moment of Silence", Count: 2}, {Name: "Revenant Patriarch", Count: 3}, {Name: "Ivory Gargoyle", Count: 4}, {Name: "Shisato, Whispering Hunter", Count: 3},
			{Name: "Stonehorn Dignitary", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 2},
			{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
		},
		{ // G/R: the own-draw skips
			{Name: "Elfhame Sanctuary", Count: 4}, {Name: "Molten Firebird", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
		{ // U/B: each opponent skips their untap step
			{Name: "Brine Elemental", Count: 4}, {Name: "Fatigue", Count: 3}, {Name: "Avizoa", Count: 4}, {Name: "Shisato, Whispering Hunter", Count: 3},
			{Name: "Air Elemental", Count: 4}, {Name: "Typhoid Rats", Count: 4}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
		},
	}
}

// skipShapeCounts is what a game's gorge log shows of the skips, by step.
type skipShapeCounts struct {
	armed    map[state.Step]int // SkipPhase grants (Amount >= 0)
	consumed map[state.Step]int // SkipPhase consumptions (Amount -1): the step or phase did not happen
}

func (c *skipShapeCounts) add(g *GorgeSide) {
	for _, e := range g.E.L.Events {
		if e.Kind != events.SkipPhase {
			continue
		}
		if e.Amount < 0 {
			c.consumed[e.Step]++
		} else {
			c.armed[e.Step]++
		}
	}
}

func newSkipShapeCounts() *skipShapeCounts {
	return &skipShapeCounts{armed: map[state.Step]int{}, consumed: map[state.Step]int{}}
}

// playSkipShapes plays one seed of the first n of the four decks.
func playSkipShapes(t *testing.T, seats int, seed uint64, maxTurns int32, sum *skipShapeCounts) Result {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("skipphase")
	if err != nil {
		t.Fatal(err)
	}
	decks := skipShapeDecks()
	lists := make([][]cardset.Entry, seats)
	for i := range lists {
		n := 0
		for _, e := range decks[i] {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = decks[i]
	}
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])}, Seed: seed, PassElision: true, Surface: "full"}
	if seats > 2 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(decks[2]), expand(decks[3])}
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	if sum != nil {
		rc.Finish = func(g *GorgeSide) { sum.add(g) }
	}
	var k Side
	if seats > 2 {
		tb, _, err := set.MatchupN(lists)
		if err != nil {
			t.Fatal(err)
		}
		k, err = SetSide(set, tb, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	} else {
		tb, _, _, err := set.Matchup(lists[0], lists[1])
		if err != nil {
			t.Fatal(err)
		}
		k, err = SetSide(set, tb, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
}

func runSkipShapes(t *testing.T, seats int) {
	t.Helper()
	sum := newSkipShapeCounts()
	for seed := uint64(1); seed <= 12; seed++ {
		res := playSkipShapes(t, seats, seed, 30, sum)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q\nref %q\nker %q\n%s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	t.Logf("%d seats: armed %v consumed %v", seats, sum.armed, sum.consumed)
	for _, st := range []state.Step{state.StepUntap, state.StepDraw, state.StepBeginCombat} {
		if sum.armed[st] == 0 || sum.consumed[st] == 0 {
			t.Errorf("%d seats, step %s: %d skips armed, %d consumed: want both above zero", seats, st, sum.armed[st], sum.consumed[st])
		}
	}
}

// TestSkipPhaseShapesFourSeatLockstep: four seats, the skips armed at a
// player who is not the active one and consumed on that player's own turn.
func TestSkipPhaseShapesFourSeatLockstep(t *testing.T) {
	runSkipShapes(t, 4)
}

// TestSkipPhaseShapesTwoSeatLockstep is the same at two seats.
func TestSkipPhaseShapesTwoSeatLockstep(t *testing.T) {
	runSkipShapes(t, 2)
}

// TestSkipPhaseUntapMutation: a kernel that consumes an untap skip but still
// untaps must diverge from gorge, which leaves the permanents tapped.
func TestSkipPhaseUntapMutation(t *testing.T) {
	dirac.DebugSkipUntapIgnored = true
	defer func() { dirac.DebugSkipUntapIgnored = false }()
	for seed := uint64(1); seed <= 12; seed++ {
		if res := playSkipShapes(t, 2, seed, 30, nil); res.Div != nil {
			t.Logf("mutation diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("DebugSkipUntapIgnored: no divergence in 12 two-seat games")
}
