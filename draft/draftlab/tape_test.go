package draftlab

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// tapeEntrants builds two fdn-limited catalog entrants with the named
// players, for the tape tests.
func tapeEntrants(t *testing.T, l *Lab, nameA, nameB string) (a, b Entrant, decks []Deck) {
	t.Helper()
	decks, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		t.Fatalf("CatalogDecks: %v", err)
	}
	if len(decks) < 2 {
		t.Fatalf("%d catalog decks, want at least 2", len(decks))
	}
	a = Entrant{Deck: decks[0], Player: testPlayer(t, l, nameA)}
	b = Entrant{Deck: decks[1], Player: testPlayer(t, l, nameB)}
	return a, b, decks
}

// TestTapeOffByDefault: without Options.RecordTape a GameResult carries no
// tape.
func TestTapeOffByDefault(t *testing.T) {
	l := testLab(t)
	a, b, _ := tapeEntrants(t, l, "bot", "lethal-pressure")
	res, err := l.PlayGame(a, b, 71)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if res.Tape != nil {
		t.Fatal("GameResult.Tape is set without Options.RecordTape")
	}
}

// TestReplayTapeReproducesHead: a recorded game re-drives with no player
// policy to the same chain head, across seeds.
func TestReplayTapeReproducesHead(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true})
	for _, seed := range []uint64{101, 202, 303} {
		a, b, decks := tapeEntrants(t, l, "bot", "lethal-pressure")
		res, err := l.PlayGame(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: PlayGame: %v", seed, err)
		}
		if res.Tape == nil {
			t.Fatalf("seed %d: PlayGame recorded no tape", seed)
		}
		if len(res.Tape.Intents) != res.Intents {
			t.Fatalf("seed %d: tape holds %d intents, result says %d", seed, len(res.Tape.Intents), res.Intents)
		}
		if res.Tape.Mulligans != l.mulliganRound() {
			t.Fatalf("seed %d: tape mulligan round %d, lab runs %d", seed, res.Tape.Mulligans, l.mulliganRound())
		}
		if len(res.Tape.Intents) == 0 {
			t.Fatalf("seed %d: the tape is empty; the head assertion would be vacuous", seed)
		}
		head, err := l.ReplayTape(decks[0], decks[1], seed, res.Tape)
		if err != nil {
			t.Fatalf("seed %d: ReplayTape: %v", seed, err)
		}
		if head != res.Head {
			t.Fatalf("seed %d: replayed head %s, recorded %s", seed, head, res.Head)
		}
	}
}

// TestReplayTapeRejectsTamper: a flipped choice, a dropped last intent and
// an appended intent all refuse to replay.
func TestReplayTapeRejectsTamper(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true})
	a, b, decks := tapeEntrants(t, l, "bot", "lethal-pressure")
	res, err := l.PlayGame(a, b, 404)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	base := *res.Tape

	// Strict replay: every recorded intent is submitted verbatim and any
	// engine rejection refuses the tape — there is no fallback re-answer, so
	// the flip may target the FIRST intent, not only post-pregame ones. Flip
	// the pregame asks (a starting_player ask at intent 0, a mulligan ask at
	// intent 1) to a choice index no ask offers: each must error naming the
	// engine's out-of-range rejection, never replay to the recorded head.
	for i, want := range []string{"kind starting_player", "kind mulligan"} {
		flipped := base
		flipped.Intents = append([]decision.Intent(nil), base.Intents...)
		if i >= len(flipped.Intents) {
			t.Fatalf("the tape has %d intents, want one to flip for %s", len(flipped.Intents), want)
		}
		if len(flipped.Intents[i].Choices) == 0 {
			t.Fatalf("intent %d recorded no choices; the flip would be vacuous", i)
		}
		flipped.Intents[i].Choices = append([]int(nil), flipped.Intents[i].Choices...)
		flipped.Intents[i].Choices[0] = 1048576
		_, err := l.ReplayTape(decks[0], decks[1], 404, &flipped)
		if err == nil {
			t.Errorf("intent %d flipped to 1048576 replayed cleanly, want a %s rejection", i, want)
			continue
		}
		if !strings.Contains(err.Error(), "out of range") || !strings.Contains(err.Error(), want) {
			t.Errorf("intent %d flipped to 1048576: error %v, want an out-of-range %s rejection", i, err, want)
		}
	}

	// A flipped post-pregame choice refuses the same way (a fallback re-answer
	// would only sometimes diverge here): flip the first intent after the pregame asks.
	flipped := base
	flipped.Intents = append([]decision.Intent(nil), base.Intents...)
	for i := 2; i < len(flipped.Intents); i++ {
		if len(flipped.Intents[i].Choices) > 0 {
			flipped.Intents[i].Choices = append([]int(nil), flipped.Intents[i].Choices...)
			flipped.Intents[i].Choices[0] = 1048576
			break
		}
	}
	if _, err := l.ReplayTape(decks[0], decks[1], 404, &flipped); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("a post-pregame intent flipped to 1048576: error %v, want an out-of-range rejection", err)
	}

	// Flip one recorded MID-GAME choice to its complement: the replay either
	// errors (a rejected or diverged answer) or reaches a different head —
	// never the same head.
	flippedMid := base
	flippedMid.Intents = append([]decision.Intent(nil), base.Intents...)
	for i := len(flippedMid.Intents) / 2; i < len(flippedMid.Intents); i++ {
		if len(flippedMid.Intents[i].Choices) > 0 {
			flippedMid.Intents[i].Choices = append([]int(nil), flippedMid.Intents[i].Choices...)
			flippedMid.Intents[i].Choices[0] ^= 1
			break
		}
	}
	head, err := l.ReplayTape(decks[0], decks[1], 404, &flippedMid)
	if err == nil && head == res.Head {
		t.Fatal("a flipped choice replayed to the recorded head")
	}

	// Drop the last intent: the re-drive needs it, so it errors.
	dropped := base
	dropped.Intents = base.Intents[:len(base.Intents)-1]
	if _, err := l.ReplayTape(decks[0], decks[1], 404, &dropped); err == nil {
		t.Fatal("a one-intent-short tape replayed cleanly")
	}

	// Append one intent: the re-drive leaves it unused.
	appended := base
	appended.Intents = append(append([]decision.Intent(nil), base.Intents...), base.Intents[len(base.Intents)-1])
	_, err = l.ReplayTape(decks[0], decks[1], 404, &appended)
	if err == nil || !strings.Contains(err.Error(), "unused intents") {
		t.Errorf("an appended intent: error %v, want the unused-intents refusal", err)
	}

	// Nil tape is refused outright.
	if _, err := l.ReplayTape(decks[0], decks[1], 404, nil); err == nil {
		t.Fatal("a nil tape replayed cleanly")
	}
}

// TestReplayTapeMulliganMismatch: a tape recorded under a different London
// mulligan round is refused — the mulligan round is part of the engine
// config, so a mismatched tape cannot re-drive the game.
func TestReplayTapeMulliganMismatch(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true})
	a, b, decks := tapeEntrants(t, l, "bot", "lethal-pressure")
	res, err := l.PlayGame(a, b, 404)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	mull := *res.Tape
	mull.Mulligans = l.mulliganRound() + 1
	if _, err := l.ReplayTape(decks[0], decks[1], 404, &mull); err == nil || !strings.Contains(err.Error(), "mulligan") {
		t.Errorf("a mulligan-mismatched tape: error %v, want the mulligan refusal", err)
	}
}

// TestReplayTapeStalledGame: a watchdog-stalled game's tape verifies by head
// too.
func TestReplayTapeStalledGame(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true, MaxTurns: 3})
	es := testEntrants(t, l, 2)
	res, err := l.PlayGame(es[0], es[1], 5)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if res.Stall != "turns" {
		t.Fatalf("stall %q, want turns (MaxTurns 3)", res.Stall)
	}
	if res.Tape == nil || len(res.Tape.Intents) == 0 {
		t.Fatal("the stalled game recorded no tape; the head assertion would be vacuous")
	}
	head, err := l.ReplayTape(es[0].Deck, es[1].Deck, 5, res.Tape)
	if err != nil {
		t.Fatalf("ReplayTape: %v", err)
	}
	if head != res.Head {
		t.Fatalf("replayed head %s, recorded %s", head, res.Head)
	}
}

// TestReplayTapeEnvPlayer: a game recorded under the bench Env driver
// (sb-tactical, the AllowDecklistBots opt-in) re-drives with plain hooks and
// the tape alone to the same head — the claim that payment-action
// and target-hint offers are pure derived reads, so a tape seat need not
// mirror the original seat's Wants* flags.
func TestReplayTapeEnvPlayer(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true, AllowDecklistBots: true})
	a, b, decks := tapeEntrants(t, l, "sb-tactical", "bot")
	res, err := l.PlayGame(a, b, 808)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if res.Tape == nil || len(res.Tape.Intents) == 0 {
		t.Fatal("the Env-driven game recorded no tape; the head assertion would be vacuous")
	}
	head, err := l.ReplayTape(decks[0], decks[1], 808, res.Tape)
	if err != nil {
		t.Fatalf("ReplayTape: %v", err)
	}
	if head != res.Head {
		t.Fatalf("replayed head %s, recorded %s", head, res.Head)
	}
}

// TestVerifyEventTapes: an event run with RecordTape verifies from its tapes
// alone; a tampered GameEnded head and a missing tape fail; VerifyEvent on
// the same result still passes.
func TestVerifyEventTapes(t *testing.T) {
	l := testLabOpts(t, Options{RecordTape: true, Workers: 4})
	s := resolveEventAgents(t, l, testEventSpec(42))
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	if len(res.Games) == 0 {
		t.Fatal("the event played no games; the tape assertions would be vacuous")
	}
	for i, g := range res.Games {
		if g.Tape == nil || len(g.Tape.Intents) == 0 {
			t.Fatalf("game %d recorded no tape", i)
		}
	}
	if err := l.VerifyEventTapes(res); err != nil {
		t.Fatalf("VerifyEventTapes: %v", err)
	}

	// VerifyEvent, the players-driven path, still passes on the same result.
	player := testPlayer(t, l, "bot")
	if err := l.VerifyEvent(res, []NamedPlayer{player}); err != nil {
		t.Fatalf("VerifyEvent: %v", err)
	}

	// A tampered GameEnded head fails.
	tampered := res
	tampered.Log = alterFirstGameHead(t, res.Log)
	if err := l.VerifyEventTapes(tampered); err == nil {
		t.Fatal("VerifyEventTapes accepted a tampered GameEnded head")
	}

	// A result with one tape set to nil fails with the no-tape refusal.
	games := append([]GameResult(nil), res.Games...)
	games[0].Tape = nil
	noTape := res
	noTape.Games = games
	err = l.VerifyEventTapes(noTape)
	if err == nil || !strings.Contains(err.Error(), "no tape") || !strings.Contains(err.Error(), "RecordTape") {
		t.Errorf("a nil-tape result: error %v, want the no-tape refusal naming RecordTape", err)
	}
}
