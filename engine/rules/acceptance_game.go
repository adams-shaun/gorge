package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The deterministic acceptance game is the one game per seat count whose
// finished chain head TestHeads pins (engine/rules/testdata/heads/<seats>.txt) and
// TestRepoDecksPlayAtEverySeatCount checks invariants and replay against. It
// lives in a non-test file so that cmd/headdiff plays exactly the same game
// as the tests: a reviewer comparing two builds' event streams must be
// comparing the games the goldens pin, not a copy that could drift.
//
// The seating (the 12 pinned Legacy decks, round-robined from deck 0) is
// testkit/testutil's AcceptanceDecks, because the deck pool is testutil's
// and production rules may not import it. The bot is botpolicy.GameBot,
// passed in as a factory because rules may not import botpolicy either
// (botpolicy's own tests import rules, so the edge would be a test import
// cycle). A caller composes them:
//
//	names, decks, err := testutil.AcceptanceDecks(reg, seats)
//	cfg := rules.AcceptanceConfig(reg, names, decks)
//	e, n, err := rules.PlayAcceptance(cfg, func(seed uint64) rules.Answerer {
//		b := botpolicy.NewGameBot(seed)
//		return func(e *rules.Engine, d *decision.Decision) decision.Intent { return b.Answer(e.G, e, d) }
//	}, nil)
const (
	// AcceptanceSeed is the acceptance game's Config.Seed.
	AcceptanceSeed uint64 = 42
	// acceptanceBotSeed seeds the one bot that answers every seat.
	acceptanceBotSeed uint64 = 7
	// acceptanceIntentCap bounds the game loop; a game that reaches it has
	// not finished, which PlayAcceptance reports as an error.
	acceptanceIntentCap = 400000
	// acceptanceStepEvery is how often (in intents) PlayAcceptance calls its
	// step hook between the first and last call.
	acceptanceStepEvery = 997
)

// AcceptanceSeatCounts returns the seat counts the acceptance games are
// played and pinned at, in order.
func AcceptanceSeatCounts() []int { return []int{2, 4, 6, 8} }

// AcceptanceConfig returns the acceptance game's Config for decks already
// seated and resolved (names[i] is decks[i]'s deck name).
func AcceptanceConfig(reg *cards.Registry, names []string, decks [][]*cards.Card) Config {
	return Config{Seed: AcceptanceSeed, Names: names, Decks: decks, Tokens: reg.Tokens, NameUniverse: reg.Universe(),
		// The mulligan is NOT configurable off for the acceptance
		// decks -- a mulligan the suite never exercises is a mulligan nobody
		// tests. Mulligans = 1 makes the keep/mulligan and bottoming round run
		// in every acceptance game; standalone fixture Configs never set it,
		// so the zero value runs no such round for them.
		Mulligans: 1}
}

// CommanderAcceptanceSeed is the seed of the 4-seat Commander acceptance
// game TestHeads pins as TestHeads/cmd4. It is separate from AcceptanceSeed
// because a Commander pod and a constructed table are different games; the
// pinned seed was measured to finish by a win (see TestCommanderHeadGameEnds).
const CommanderAcceptanceSeed uint64 = 42

// CommanderAcceptanceConfig returns the 4-seat Commander acceptance game's
// Config for decks already seated and resolved, with commanders[i] naming
// seat i's commander indices into decks[i]. FormatCommander leaves
// StartingLife at the format default (40, CR 903.7), so the head pins the
// engine-default life total. The mulligan round is on for the same reason
// AcceptanceConfig keeps it on: the pinned game exercises keep/mulligan and
// bottoming.
func CommanderAcceptanceConfig(reg *cards.Registry, names []string, decks [][]*cards.Card, commanders [][]int) Config {
	return Config{Seed: CommanderAcceptanceSeed, Names: names, Decks: decks, Commanders: commanders,
		Format: FormatCommander, Tokens: reg.Tokens, NameUniverse: reg.Universe(), Mulligans: 1}
}

// Answerer answers one pending decision of e.
type Answerer func(e *Engine, d *decision.Decision) decision.Intent

// PlayAcceptance plays cfg to completion the acceptance way: CR 103.1's
// winner-chooses starting-player ask is posed (the choice constructor defers
// the pregame rounds, and the pose lets the bot answer it, the same decision
// a real table conveys to the toss winner), and one bot, newBot called with
// the acceptance bot seed, answers every decision. newBot must return
// botpolicy.GameBot's answers (engine/rules/testbot_test.go's testBot and
// cmd/headdiff both do); any other bot plays a different game. It returns
// the finished engine and the number of intents submitted.
//
// step, when non-nil, is called once right after Advance (n==0, before any
// intent), once every acceptanceStepEvery intents thereafter, and once more
// after the game loop exits, so a caller that wants per-checkpoint work
// (invariant checks, logging) hooks into the one game loop instead of
// keeping a second copy of it.
//
// A refused intent or a game that has not finished within the intent cap is
// an error; the returned engine is then the game as far as it got.
func PlayAcceptance(cfg Config, newBot func(seed uint64) Answerer, step func(e *Engine, n int)) (*Engine, int, error) {
	e := NewStartingPlayerChoice(cfg)
	answer := newBot(acceptanceBotSeed)
	e.AskStartingPlayer()
	e.Advance()
	if step != nil {
		step(e, 0)
	}
	n := 0
	for !e.G.Over && e.Pending() != nil && n < acceptanceIntentCap {
		if err := e.Submit(answer(e, e.Pending())); err != nil {
			return e, n, fmt.Errorf("intent %d: %w", n, err)
		}
		if step != nil && n%acceptanceStepEvery == 0 {
			step(e, n)
		}
		n++
	}
	if step != nil {
		step(e, n)
	}
	if !e.G.Over {
		return e, n, fmt.Errorf("game did not finish (turn %d, %d intents)", e.G.Turn, n)
	}
	return e, n, nil
}
