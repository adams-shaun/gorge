package seat

import (
	"context"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
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

// TestBotAdaptersAgreePerStep pins Ruling F7's "keep the two in step" as a
// measured property, per step: for every step the engine can be in, the two
// adapter halves must build the same botpolicy.Board from the same facts --
// the view-shaped half (boardFromView, fed the Phase string view.PhaseOf
// projects for that step, which is exactly what a real seat receives) and
// the game-shaped half (the rules test host's g.Step.IsMain()). A bare
// step has no game behind it, so the only fact either half can report is
// IsMain; the creature/life census agreement is TestBotAdaptersAgreeOverWholeGame's
// territory instead. The policy must then answer the same; this uses the
// priority decision where IsMain changes the choice (in a main phase the
// policy taps mana, outside it the same options are passed on), with the
// two sides' rngs seeded identically. This is the test that dies on
// mutation M1: invert boardFromView's IsMain and the halves disagree
// precisely on StepMain1/StepMain2.
func TestBotAdaptersAgreePerStep(t *testing.T) {
	prio := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Obj: 100},
			{Index: 1, Kind: "pass"},
		}}
	for _, s := range allSteps {
		boardView := boardFromView(view.View{Phase: view.PhaseOf(s)})
		boardGame := botpolicy.Board{IsMain: s.IsMain()} // the rules host's expression
		if boardView.IsMain != boardGame.IsMain {
			t.Errorf("step %s: view-shaped IsMain %v, game-shaped IsMain %v", s, boardView.IsMain, boardGame.IsMain)
		}
		inView, err := NewBot(1).Decide(context.Background(), view.View{Phase: view.PhaseOf(s)}, prio)
		if err != nil {
			t.Fatalf("step %s: view-shaped Decide: %v", s, err)
		}
		inGame := botpolicy.Decide(boardGame, &prio, rand.New(rand.NewPCG(1, 1^0x9e3779b97f4a7c15)))
		if !slices.Equal(inView.Choices, inGame.Choices) {
			t.Errorf("step %s: view-shaped choices %v, game-shaped choices %v", s, inView.Choices, inGame.Choices)
		}
	}
}

// TestBotAdaptersAgreeOverWholeGame drives two byte-identical acceptance
// games from the same (engine seed, bot seed) -- one through this package's
// view-shaped adapter (projecting a View every decision, exactly like a real
// client), one through the rules host's game-shaped adapter (e.G.Step.IsMain
// on the engine's own step). Every intent the two produce must be identical
// -- same Seq/Player/Choices -- so the two chains reach the same head. This
// is the copy-paste mirror's guarantee (Ruling F7) turned into a measured
// property of the two real adapter halves over a whole game.
func TestBotAdaptersAgreeOverWholeGame(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	cfg := rules.Config{Seed: 0, Names: names, Decks: decks}
	eView := rules.New(cfg)
	eGame := rules.New(cfg)
	eView.Advance()
	eGame.Advance()
	botView := NewBot(7)
	botGame := rand.New(rand.NewPCG(7, 7^0x9e3779b97f4a7c15))
	n := 0
	for !eView.G.Over && !eGame.G.Over && eView.Pending() != nil && eGame.Pending() != nil && n < 200000 {
		d := eView.Pending()
		inView, err := botView.Decide(context.Background(), view.Project(eView.G, eView, d.Player, d), *d)
		if err != nil {
			t.Fatalf("intent %d: view-shaped Decide: %v", n, err)
		}
		// The game-shaped half builds the same Board straight off the engine
		// (botpolicy.BoardFromGame: IsMain from eGame.G.Step, the creature
		// census and life from eGame.G) -- the exact expression the rules
		// test host's answer uses, so a divergence between the two adapter
		// halves fails here on the intent where it first appears.
		boardGame := botpolicy.BoardFromGame(eGame.G, eGame, d.Player)
		// The casting Card census (B4): the view-shaped half fills the same
		// map for the same deciding player off the projected Hand/Graveyard/
		// Battlefield CardViews. It is the new field this task's widening adds
		// to Board, so it is pinned here explicitly -- not only through the
		// intents below (which would catch a divergence only when a ranking
		// actually flips a choice): a Cards map one half fills and the other
		// leaves zero is a bot that casts differently depending on who asked.
		boardView := boardFromView(view.Project(eView.G, eView, d.Player, d))
		if !maps.Equal(boardView.Cards, boardGame.Cards) {
			t.Fatalf("intent %d: casting Card census diverged: view %v vs game %v (step %s)", n, boardView.Cards, boardGame.Cards, eGame.G.Step)
		}
		inGame := botpolicy.Decide(boardGame, eGame.Pending(), botGame)
		if inView.Seq != inGame.Seq || inView.Player != inGame.Player || !slices.Equal(inView.Choices, inGame.Choices) {
			t.Fatalf("intent %d: adapters diverged: view %+v vs game %+v (step %s)", n, inView, inGame, eGame.G.Step)
		}
		if err := eView.Submit(inView); err != nil {
			t.Fatalf("intent %d: view-shaped Submit: %v", n, err)
		}
		if err := eGame.Submit(inGame); err != nil {
			t.Fatalf("intent %d: game-shaped Submit: %v", n, err)
		}
		n++
	}
	if !eView.G.Over || !eGame.G.Over {
		t.Fatalf("game did not terminate after %d intents (view over=%v, game over=%v)", n, eView.G.Over, eGame.G.Over)
	}
	if h1, h2 := eView.L.Head(), eGame.L.Head(); h1 != h2 {
		t.Fatalf("chains diverged: view %s, game %s", h1, h2)
	}
}
