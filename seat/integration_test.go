package seat

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// allSteps is every state.Step, in engine order, used only to print the
// per-step activation histogram below in a fixed, deterministic order --
// never to decide anything about a live game.
var allSteps = []state.Step{
	state.StepUntap, state.StepUpkeep, state.StepDraw, state.StepMain1,
	state.StepBeginCombat, state.StepDeclareAttackers, state.StepDeclareBlockers,
	state.StepCombatDamage, state.StepEndCombat, state.StepMain2, state.StepEnd, state.StepCleanup,
}

// TestBotOnlyActivatesInAMainPhase is Ruling T25-g's regression test against
// a real, whole game: fix round 1's own synthetic "priority (combat)" case
// (bot_test.go) carried a play_land option, which the switch's un-gated
// play_land/cast loop matches regardless of phase -- a shape the live
// engine never actually offers outside a main phase (play_land requires
// sorcery speed), so that test never reached the clamp-top-up fallback path
// that reintroduced I-1(b) in real games. This drives one whole
// SampleDecks(t,4) game with the real seat.Bot against the real engine
// (seat is allowed to import rules for its own tests: rules does not import
// seat, so there is no cycle -- the same pattern view_test.go already uses)
// and tallies every "activate" choice by the step it was made in, reading
// e.G.Step before each Submit.
func TestBotOnlyActivatesInAMainPhase(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	e := rules.New(rules.Config{Seed: 0, Names: names, Decks: decks})
	e.Advance()
	b := NewBot(0)

	hist := map[state.Step]int{}
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 200000 {
		d := e.Pending()
		step := e.G.Step
		v := view.Project(e.G, e, d.Player, d)
		in, err := b.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: Decide returned an error: %v", n, err)
		}
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

	t.Log("seat.Bot activate choices by step:")
	for _, s := range allSteps {
		t.Logf("  %-18s %d", s, hist[s])
	}
	for _, s := range allSteps {
		if s == state.StepMain1 || s == state.StepMain2 {
			continue
		}
		if hist[s] > 0 {
			t.Errorf("seat.Bot activated %d times during %s, outside any main phase", hist[s], s)
		}
	}
	if hist[state.StepMain1]+hist[state.StepMain2] == 0 {
		t.Error("seat.Bot never activated during a main phase across the whole game")
	}
}
