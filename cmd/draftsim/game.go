package main

import (
	"fmt"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// The game watchdogs: a limited game that reaches either cap is recorded as a watchdog draw in
// its series (a GameEnded with no winner), while a livelock or a recovered
// engine panic is a hard error — an engine bug, not a slow game.
const (
	gameMaxTurns   = 150
	gameMaxIntents = 40000
	// draftMulligans is the London mulligan round each seat may take (a
	// 40-card deck with ~17 lands wants one more often than constructed);
	// the keep/mulligan answers come from botpolicy.MulliganLands.
	draftMulligans = 2
)

// gameOutcome is one driven game's engine-level result, in the shape both
// callers consume: the event's round-robin (playGame) and the gauntlet's
// measurement games (playSeeded's direct callers).
type gameOutcome struct {
	// winner is the winning PLAYER index, 0 for the first-listed seat and
	// 1 for the second; nil for a draw or a watchdog stall.
	winner *int
	// stall is the watchdog that capped the game, "" for a finished one.
	stall string
	// turns, intents and head are the chain facts -verify re-checks.
	turns   int
	intents int
	head    string
}

// playGame plays one Bo1 game between event seats a and b, deriving the
// engine seed the way every event game does, and with verify set replays
// the chain afterwards. It returns the game's record with its chain head;
// the winner is the event seat index, nil for a draw or a watchdog stall.
func playGame(r *runner, eventSeed uint64, round, a, b, game int, deckA, deckB []*cards.Card, verify bool) (gameRecord, error) {
	seed := gameSeed(eventSeed, round, a, b, game)
	oc, err := playSeeded(r, seed, [2]string{r.players[a], r.players[b]},
		[2]string{fmt.Sprintf("p%d", a), fmt.Sprintf("p%d", b)}, deckA, deckB, verify)
	if err != nil {
		return gameRecord{}, err
	}
	rec := gameRecord{
		head:     oc.head,
		turns:    oc.turns,
		intents:  oc.intents,
		verified: verify,
	}
	rec.stall = oc.stall
	if oc.winner != nil {
		w := a
		if *oc.winner == 1 {
			w = b
		}
		rec.winner = &w
	}
	return rec, nil
}

// playSeeded plays one Bo1 game with an explicit engine seed, the two
// seats' policies, display names and decks, and with verify set replays
// the chain afterwards. It is the shared body of playGame (the event's
// round-robin games, seed derived from the event) and the gauntlet's
// measurement games (seed derived from the gauntlet's own salt space).
func playSeeded(r *runner, seed uint64, players, names [2]string, deckA, deckB []*cards.Card, verify bool) (gameOutcome, error) {
	oc, eng, err := driveGame(r, seed, players, names, deckA, deckB)
	if err != nil {
		return gameOutcome{}, err
	}
	out := gameOutcome{
		head:    eng.L.Head(),
		turns:   int(oc.Turns),
		intents: oc.Intents,
	}
	switch {
	case bench.IsAbort(oc.StallOn):
		return gameOutcome{}, fmt.Errorf("seed %d: engine aborted (%s): %s", seed, oc.StallOn, firstLine(oc.Livelock))
	case oc.StallOn != "":
		out.stall = oc.StallOn
	case oc.Draw:
		// winner stays nil: a drawn game
	default:
		w := oc.WinnerSeat
		out.winner = &w
	}
	if verify && out.stall == "" {
		if _, err := replay.Replay(eng.L, gameConfig(r, seed, names, deckA, deckB)); err != nil {
			return gameOutcome{}, fmt.Errorf("seed %d: replay: %w", seed, err)
		}
	}
	return out, nil
}

// replayGameHead re-plays one logged game from the log-rebuilt event's
// saved decks and returns the chain head it produces, for -verify's
// head-for-head comparison.
func replayGameHead(r *runner, ev *event.Event, round, a, b, game int) (string, error) {
	deckA, err := resolveSavedDeck(r, ev, a)
	if err != nil {
		return "", fmt.Errorf("seat %d deck: %w", a, err)
	}
	deckB, err := resolveSavedDeck(r, ev, b)
	if err != nil {
		return "", fmt.Errorf("seat %d deck: %w", b, err)
	}
	seed := gameSeed(ev.Seed(), round, a, b, game)
	oc, eng, err := driveGame(r, seed, [2]string{r.players[a], r.players[b]},
		[2]string{fmt.Sprintf("p%d", a), fmt.Sprintf("p%d", b)}, deckA, deckB)
	if err != nil {
		return "", err
	}
	if oc.StallOn != "" {
		return "", fmt.Errorf("seed %d: replay ended as a %s stall", seed, oc.StallOn)
	}
	return eng.L.Head(), nil
}

// resolveSavedDeck turns the event's saved deck revision into the engine's
// card pointers, the same path the live run used (SavedDeck → ToDeckFile →
// Resolve), so a replayed game reads the same 40 cards in the same order.
func resolveSavedDeck(r *runner, ev *event.Event, seatIdx int) ([]*cards.Card, error) {
	d, ok := ev.SavedDeck(seatIdx)
	if !ok {
		return nil, fmt.Errorf("seat %d has no saved deck in the log", seatIdx)
	}
	pool := ev.Pool(seatIdx)
	f := build.ToDeckFile(pool, d, fmt.Sprintf("p%d %s draft", seatIdx, r.spec.Code))
	return f.Resolve(r.reg)
}

// driveGame builds the two seats and the engine config for one game and
// drives it to completion (or to a watchdog cap) on the caller's own
// goroutine, exactly cmd/mtgsim's drive loop's contract. players names the
// bot policy for each player and names the deck display names the engine
// config carries; both are indexed by player.
func driveGame(r *runner, seed uint64, players, names [2]string, deckA, deckB []*cards.Card) (bench.Outcome, *rules.Engine, error) {
	seats := make([]seat.Seat, 2)
	for i, p := range players {
		st, err := bots.New(p, bots.Options{
			Seed:        seed ^ uint64(i+1),
			AutoPayMana: true,
			Deps:        bots.Deps{Cards: r.reg},
		})
		if err != nil {
			return bench.Outcome{}, nil, fmt.Errorf("seat %d policy %q: %w", i, p, err)
		}
		// The limited mulligan: the plain policies all build on *heuristic.Bot,
		// whose London rule is opt-in per seat. A policy that some day
		// wraps its bot loses the heuristic and keeps the default coin.
		if bot, ok := st.(*heuristic.Bot); ok {
			bot.WithMulligan(botpolicy.MulliganLands)
		}
		seats[i] = st
	}
	oc, eng, err := bench.PlayGame(gameConfig(r, seed, names, deckA, deckB), seats, gameMaxTurns, gameMaxIntents, bench.Hooks{
		// The pregame ask, the way cmd/mtgsim's playOne poses it (the
		// brief's Facts line): the engine is built with
		// rules.NewStartingPlayerChoice, which defers the pregame rounds,
		// and Setup poses CR 103.1's winner-chooses decision before the
		// first Advance. botpolicy.Decide answers it deterministically
		// (the toss winner names itself), so the recorded Intent replays
		// through the log's DecisionAsk.
		NewEngine: rules.NewStartingPlayerChoice,
		Setup:     func(e *rules.Engine) { e.AskStartingPlayer() },
	})
	if err != nil {
		return bench.Outcome{}, nil, fmt.Errorf("seed %d: %w", seed, err)
	}
	return oc, eng, nil
}

// gameConfig is one game's engine config: the two decks, the derived seed
// and the name universe the engine's NameCard asks may need. names carries
// each player's deck display name (the same "p0"/"p1" labels the event
// games always used).
func gameConfig(r *runner, seed uint64, names [2]string, deckA, deckB []*cards.Card) rules.Config {
	return rules.Config{
		Seed:         seed,
		Names:        []string{names[0], names[1]},
		Decks:        [][]*cards.Card{deckA, deckB},
		Tokens:       r.reg.Tokens,
		NameUniverse: r.reg.Universe(),
		Mulligans:    draftMulligans,
	}
}

// firstLine cuts a diagnostic to its first line, for the game-abort error.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
