package rules

import (
	"fmt"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// priorityHistSteps is every state.Step, in engine order, used only to
// print the per-step activation histogram below in a fixed, deterministic
// order -- never to decide anything about a live game. (Named distinctly
// from engine/seat/integration_test.go's allSteps: same idea, different package,
// no shared declaration to keep in sync.)
var priorityHistSteps = []state.Step{
	state.StepUntap, state.StepUpkeep, state.StepDraw, state.StepMain1,
	state.StepBeginCombat, state.StepDeclareAttackers, state.StepDeclareBlockers,
	state.StepCombatDamage, state.StepEndCombat, state.StepMain2, state.StepEnd, state.StepCleanup,
}

// TestTestBotOnlyActivatesInAMainPhase is engine/seat/integration_test.go's
// TestBotOnlyActivatesInAMainPhase, driven against this package's own
// mirror instead of the real heuristic.Bot: same regression,
// same real engine shapes, driving one whole SampleDecks(t,4) game
// to completion and tallying every "activate" choice by the step it was
// made in (e.G.Step read before each Submit) -- so a divergence between the
// two mirrors on this exact defect would show up here even if it somehow
// didn't in the seat package's own test.
func TestTestBotOnlyActivatesInAMainPhase(t *testing.T) {
	t.Parallel()
	names, decks := testutil.SampleDecks(t, 4)
	e := New(Config{Seed: 0, Names: names, Decks: decks})
	e.Advance()
	b := newTestBot(0)

	hist := map[state.Step]int{}
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 200000 {
		d := e.Pending()
		step := e.G.Step
		in := b.answer(e, d)
		if d.Kind == decision.KPriority {
			if chosen := d.Chosen(in); len(chosen) == 1 && chosen[0].Kind == "activate" {
				hist[step]++
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents (turn %d)", n, e.G.Turn)
	}

	t.Log("testBot activate choices by step:")
	for _, s := range priorityHistSteps {
		t.Logf("  %-18s %d", s, hist[s])
	}
	for _, s := range priorityHistSteps {
		if s == state.StepMain1 || s == state.StepMain2 {
			continue
		}
		if hist[s] > 0 {
			t.Errorf("testBot activated %d times during %s, outside any main phase", hist[s], s)
		}
	}
	if hist[state.StepMain1]+hist[state.StepMain2] == 0 {
		t.Error("testBot never activated during a main phase across the whole game")
	}
}

// TestBotMatchIsDeterministicAcrossRuns confirms the pass rule does
// not disturb determinism: the same (engine seed, bot
// seed) pair, driven to completion twice, must produce the identical event
// log -- Log.Head() folds in every event's full content, so an equal head
// after a full game is a strong statement, not merely "the first few
// decisions matched".
func TestBotMatchIsDeterministicAcrossRuns(t *testing.T) {
	t.Parallel()
	// The two runs are independent games, so they play at once. SampleDecks
	// runs on the test goroutine; the games only return errors.
	names, decks := testutil.SampleDecks(t, 4)
	type result struct {
		head   string
		events int
		err    error
	}
	run := func() result {
		e := New(Config{Seed: 3, Names: names, Decks: decks})
		b := newTestBot(3 * 31)
		e.Advance()
		n := 0
		for !e.G.Over && e.Pending() != nil && n < 200000 {
			if err := e.Submit(b.answer(e, e.Pending())); err != nil {
				return result{err: fmt.Errorf("intent %d: %w", n, err)}
			}
			n++
		}
		if !e.G.Over {
			return result{err: fmt.Errorf("game did not terminate after %d intents", n)}
		}
		return result{head: e.L.Head(), events: len(e.L.Events)}
	}
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = run()
		}(i)
	}
	wg.Wait()
	for _, r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
	}
	head1, n1 := results[0].head, results[0].events
	head2, n2 := results[1].head, results[1].events
	if head1 != head2 || n1 != n2 {
		t.Fatalf("same (engine seed, bot seed) produced different runs: head=%s/%s events=%d/%d",
			head1, head2, n1, n2)
	}
	t.Logf("deterministic: head=%s events=%d", head1, n1)
}
