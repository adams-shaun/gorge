package replay

import (
	"context"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// botAnswer is the replay harness's one shared way of getting a
// deterministic bot to answer a decision without projecting a full View.
// It is the exact game-shaped adapter the real host loop already uses for a
// BoardSeat (host/match.go's projectNext builds the botpolicy.Board from the
// engine under the lock and decideInto calls DecideBoard) and the rules
// fuzz host uses (rules/testbot_test.go's answer: botpolicy.BoardFromGame +
// botpolicy.Decide): the botpolicy.Board is built straight off the engine
// (botpolicy.BoardFromGame) and forwarded unchanged through seat.Bot's own
// DecideBoard, so there is no view.cardViews string round-trip anywhere in
// the drive. A seat.Bot's DecideBoard and Decide consume the same policy
// over the same rng stream, and boardFromView and BoardFromGame build the
// same Board from the same game facts (seat's
// TestBotAdaptersAgreeOverWholeGame pins the two halves identical), so this
// returns exactly the intent the old view-shaped driver — view.Project then
// b.Decide — would have returned, which is what keeps the replay chains in
// this package byte-identical to before. Every replay test drives bots
// through this one helper.
func botAnswer(b *seat.Bot, e *rules.Engine, d *decision.Decision) (decision.Intent, error) {
	return b.DecideBoard(context.Background(), botpolicy.BoardFromGame(e.G, e, d.Player), *d)
}

// TestBotGameDriverMatchesProjectedView is the named regression for the
// invariant botAnswer (and therefore every rewritten drive loop in this
// package) relies on: the game-shaped adapter — botpolicy.BoardFromGame fed
// to a seat.Bot's DecideBoard — must return the byte-identical intent the
// view-shaped adapter — view.Project fed to the same seat.Bot's Decide —
// returns, at every decision of a real game. It drives one complete
// 2-seat game and checks the two drivers' intents against each other at
// every pending decision.
//
// Two same-seed bots run in lockstep: each Advances its own independent PCG
// stream only through the decisions it actually makes, and because the two
// boards the drivers build from the same engine are equal (the invariant
// under test), the decisions and therefore the per-decision rng consumption
// are identical, so the streams never drift. The moment the two boards
// disagree — a battlefield creature the game-shaped census drops, a life
// total read wrong, the deciding seat's own hand read as another's — the two
// intents differ here, on the decision where it first appears, instead of
// silently replaying to a different chain head.
func TestBotGameDriverMatchesProjectedView(t *testing.T) {
	cfg := raiderConfig(t)
	e := rules.New(cfg)
	e.Advance()
	gameBot := seat.NewBot(cfg.Seed)
	viewBot := seat.NewBot(cfg.Seed)
	n := 0
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		want, err := viewBot.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: view-shaped Decide: %v", n, err)
		}
		in, err := botAnswer(gameBot, e, d)
		if err != nil {
			t.Fatalf("intent %d: game-shaped driver: %v", n, err)
		}
		if !reflect.DeepEqual(in, want) {
			t.Fatalf("intent %d (kind %s, step %s): game-shaped driver %+v != view-shaped intents %+v",
				n, d.Kind, e.G.Step, in, want)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: Submit: %v", n, err)
		}
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents", n)
	}
}
