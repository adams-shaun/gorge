package rules

// The CR 103.1 toss: rules.New draws the starting seat from the engine rng
// (one IntN over the seats, BEFORE any per-seat shuffle) unless the Config
// pins one. This file pins the toss end to end -- the census (the starting
// seat is neither fixed nor degenerate), the public toss Note, the rotated
// CR 103.5 mulligan round, replay-exactness of a tossed game, and the
// pinned-start arm (the "players agree" determination, which must consume no
// rng value at all so a pinned game's genesis stream stays byte-identical to
// the pre-toss engine's).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// tossNotes returns the game's toss Notes (there must be at most one).
func tossNotes(e *Engine) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "won the toss") {
			out = append(out, ev)
		}
	}
	return out
}

// tossedTwoSeat is a two-seat mountain-deck Config with the toss live (no
// pin). Seed 1 measurably tosses the game to seat 1.
func tossedTwoSeat(t *testing.T, seed uint64, mulligans int) Config {
	t.Helper()
	return Config{Seed: seed, Mulligans: mulligans,
		Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
}

// TestTossDeterminesTheStartingPlayerNotAlwaysSeatZero is the census: across
// 64 seeds at two seat counts the starting seat is neither pinned to 0 nor
// degenerate. At two seats both extreme outcomes must occur (binomial
// certainty at 64 draws of p=1/2 -- the assertion is "both observed", never
// an exact split, which would over-pin the rng).
func TestTossDeterminesTheStartingPlayerNotAlwaysSeatZero(t *testing.T) {
	for _, seats := range []int{2, 4} {
		census := map[state.PlayerID]int{}
		for seed := uint64(1); seed <= 64; seed++ {
			names := make([]string, seats)
			decks := make([][]*cards.Card, seats)
			for i := range names {
				names[i] = string(rune('a' + i))
				decks[i] = mountainDeck(t, 40)
			}
			e := New(Config{Seed: seed, Names: names, Decks: decks})
			census[e.G.Active]++
		}
		if len(census) < 2 {
			t.Fatalf("seats=%d: starting seat never varied across 64 seeds: %v", seats, census)
		}
		if census[0] == 64 {
			t.Fatalf("seats=%d: seat 0 started all 64 games -- the toss is not random", seats)
		}
		if seats == 2 {
			if census[0] == 0 || census[1] == 0 {
				t.Fatalf("seats=2: both extreme toss outcomes must occur at 64 seeds, got %v", census)
			}
		}
	}
}

// TestTossNoteIsEmittedExactlyOnceAndNamesTheStartingPlayer: the toss is
// public -- exactly one Note per game, carrying the winner as its Player and
// naming them in the text view/describe.go renders verbatim.
func TestTossNoteIsEmittedExactlyOnceAndNamesTheStartingPlayer(t *testing.T) {
	for _, seed := range []uint64{1, 2, 42} {
		cfg := tossedTwoSeat(t, seed, 0)
		e := New(cfg)
		notes := tossNotes(e)
		if len(notes) != 1 {
			t.Fatalf("seed=%d: %d toss Notes, want exactly 1", seed, len(notes))
		}
		if notes[0].Player != e.G.Active {
			t.Fatalf("seed=%d: toss Note names seat %d, game started at %d", seed, notes[0].Player, e.G.Active)
		}
		want := playerName(e.G, e.G.Active) + " won the toss and takes the first turn"
		if notes[0].Text != want {
			t.Fatalf("seed=%d: toss Note text %q, want %q", seed, notes[0].Text, want)
		}
	}
}

// TestTossedGameReplaysByteIdentically: a seeded two-seat game whose toss
// winner is NOT seat 0 replays byte-identically through the logged intents --
// same chain head, same RNGDraws count (Ruling P5's replay contract).
func TestTossedGameReplaysByteIdentically(t *testing.T) {
	cfg := tossedTwoSeat(t, 1, 0) // measured: seed 1 tosses to seat 1
	e := New(cfg)
	if e.G.Active != 1 {
		t.Fatalf("fixture precondition: seed 1 must toss to seat 1, got active=%d", e.G.Active)
	}
	e.Advance()
	passAll(t, e, 200)
	re, err := replayFor(cfg, e.L)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("replayed head %s, live head %s", re.L.Head(), e.L.Head())
	}
	if re.RNGDraws() != e.RNGDraws() {
		t.Fatalf("replayed rng draws %d, live %d", re.RNGDraws(), e.RNGDraws())
	}
}

// TestMulliganRoundAsksTheTossWinnerFirst is CR 103.5: the STARTING player
// declares whether to mulligan, then each other player in turn order. With
// the toss winner at seat 1 the round order is AliveFrom(1) = [1, 0], so the
// first keep/mulligan ask goes to seat 1 and the second to seat 0 -- the
// pre-toss engine asked seat 0 first unconditionally.
func TestMulliganRoundAsksTheTossWinnerFirst(t *testing.T) {
	cfg := tossedTwoSeat(t, 1, 1) // measured: seed 1 tosses to seat 1
	e := New(cfg)
	// During the pregame round G.Active still reads the genesis zero; the
	// round order is what New built, not G.Active. The toss Note names the
	// winner -- that is the fixture's precondition.
	notes := tossNotes(e)
	if len(notes) != 1 || notes[0].Player != 1 {
		t.Fatalf("fixture precondition failed: toss notes %+v", notes)
	}
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KMulligan {
		t.Fatalf("pending = %+v, want the first keep/mulligan ask", d)
	}
	if d.Player != 1 {
		t.Fatalf("first mulligan ask went to seat %d, want the toss winner (seat 1) -- CR 103.5", d.Player)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KMulligan {
		t.Fatalf("second pending = %+v, want the other seat's keep/mulligan ask", d)
	}
	if d.Player != 0 {
		t.Fatalf("second mulligan ask went to seat %d, want seat 0 (turn order after seat 1)", d.Player)
	}
}

// TestPinnedStartIsStreamNeutralAndDeterministic: a pinned start (the
// "players agree" arm of CR 103.1) consumes NO rng value -- genesis's draw
// count is exactly the per-seat shuffles' -- emits no toss Note, and begins
// at the pinned seat. Out-of-range pins degrade, never panic: AliveFrom wraps
// the pinned seat into the alive set by turn order.
func TestPinnedStartIsStreamNeutralAndDeterministic(t *testing.T) {
	// Two 40-card decks: two shuffles of 39 draws each, nothing else -- 78,
	// the pre-toss engine's exact genesis draw count.
	const pristineDraws = uint64(78)
	for _, pin := range []state.PlayerID{0, 1} {
		cfg := Config{Seed: 1, Names: []string{"a", "b"}, PinnedStart: true, StartSeat: pin,
			Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
		e := New(cfg)
		if e.G.Active != pin {
			t.Fatalf("pin=%d: game started at seat %d", pin, e.G.Active)
		}
		if draws := e.RNGDraws(); draws != pristineDraws {
			t.Fatalf("pin=%d: genesis consumed %d rng values, want %d (the toss draw must not run)", pin, draws, pristineDraws)
		}
		if notes := tossNotes(e); len(notes) != 0 {
			t.Fatalf("pin=%d: pinned start emitted %d toss Notes, want none", pin, len(notes))
		}
	}
	// An out-of-range pin wraps through AliveFrom: seat 9 in a two-seat game
	// resolves to seat 1 (the first alive seat at or after 9 in turn order),
	// deterministically, with no panic.
	cfg := Config{Seed: 1, Names: []string{"a", "b"}, PinnedStart: true, StartSeat: 9,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	if e.G.Active != 1 {
		t.Fatalf("out-of-range pin: game started at seat %d, want the AliveFrom(9) wrap to seat 1", e.G.Active)
	}
}
