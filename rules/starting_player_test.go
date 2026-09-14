package rules

// The CR 103.1 toss: rules.New draws the starting seat from the engine rng
// (one IntN over the seats, BEFORE any per-seat shuffle). This file pins the
// toss end to end -- the census (the starting seat is neither fixed nor
// degenerate), the public toss Note, the rotated CR 103.5 mulligan round,
// replay-exactness of a tossed game, and uniformity over the SURVIVORS when
// the deal itself eliminates a seat.

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

// tossedTwoSeat is a two-seat mountain-deck Config with the toss live. Seed 1
// measurably tosses the game to seat 1.
func tossedTwoSeat(t *testing.T, seed uint64, mulligans int) Config {
	t.Helper()
	return Config{Seed: seed, Mulligans: mulligans,
		Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
}

// seatZeroStart returns cfg with the smallest seed >= cfg.Seed whose CR 103.1
// toss starts seat 0. The scenario fixtures predate the toss: their
// protagonist is seat 0 -- before the toss seat 0 was always the starting
// player, so every fixture addresses seats and turns by index -- and the
// winner-chooses arm that would let a fixture name its starter is
// deliberately unbuilt (the "Known approximations" row in AGENTS.md). The
// toss draw sits BEFORE any shuffle, so it is deck-independent and the first
// acceptable seed is a pure function of the requested one; the effective seed
// travels in the returned Config, which is what a replay must be handed.
func seatZeroStart(cfg Config) Config {
	for {
		e := New(cfg)
		if e.G.Active == 0 {
			return cfg
		}
		cfg.Seed++
	}
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
		want := tossName(e.G, e.G.Active) + " won the toss and takes the first turn"
		if notes[0].Text != want {
			t.Fatalf("seed=%d: toss Note text %q, want %q", seed, notes[0].Text, want)
		}
	}
}

// TestTossNoteIsEmittedWhenOpeningDealEndsTheGame covers both early-return
// positions in New's deal loop. The determination still gets exactly one
// public Note, but its text cannot claim that the terminal game began turn 1.
func TestTossNoteIsEmittedWhenOpeningDealEndsTheGame(t *testing.T) {
	for _, shortSeat := range []int{0, 1} {
		decks := [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}
		decks[shortSeat] = mountainDeck(t, 3)
		e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: decks})
		if !e.G.Over || e.G.Turn != 0 {
			t.Fatalf("short seat %d: fixture did not end during the deal (over=%v turn=%d)", shortSeat, e.G.Over, e.G.Turn)
		}
		notes := tossNotes(e)
		if len(notes) != 1 {
			t.Fatalf("short seat %d: %d toss Notes, want exactly 1", shortSeat, len(notes))
		}
		winner := notes[0].Player
		if e.G.Players[winner].Lost {
			t.Fatalf("short seat %d: toss Note names eliminated seat %d", shortSeat, winner)
		}
		want := tossName(e.G, winner) + " won the toss; the game ended before the first turn"
		if notes[0].Text != want {
			t.Fatalf("short seat %d: toss Note text %q, want %q", shortSeat, notes[0].Text, want)
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

// TestTossIsUniformOverGenesisSurvivors is the survivor-bias regression
// test (fix round 2): when the deal itself eliminates a seat, the toss must
// stay uniform over the SURVIVORS -- not be reduced modulo the survivor
// count. Three seats with seat 0's three-card deck (it decks out during its
// opening hand, CR 704.5c) leave seats 1 and 2 alive; the pre-fix engine
// drew IntN(3) and took alive[toss%2], so two of the three toss outcomes
// mapped onto one survivor -- measured map[1:395 2:205] over 600 seeds
// (true p=1/2 would put the minority near 300, sigma ~12). The assertion is
// "both survivors observed, minority >= 240": under uniformity that bound
// sits ~5 sigma below the mean (failure probability ~1e-6), while the biased
// 1/3 mapping (minority ~200) fails it overwhelmingly.
func TestTossIsUniformOverGenesisSurvivors(t *testing.T) {
	census := map[state.PlayerID]int{}
	for seed := uint64(1); seed <= 600; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b", "c"},
			Decks: [][]*cards.Card{mountainDeck(t, 3), mountainDeck(t, 40), mountainDeck(t, 40)}})
		if !e.G.Players[0].Lost || e.G.Over {
			t.Fatalf("seed %d: fixture precondition failed (lost=%v over=%v)", seed, e.G.Players[0].Lost, e.G.Over)
		}
		census[e.G.Active]++
	}
	if census[0] != 0 {
		t.Fatalf("eliminated seat 0 started %d games", census[0])
	}
	if census[1] == 0 || census[2] == 0 {
		t.Fatalf("both survivors must be observed as the starting seat, got %v", census)
	}
	minority := census[1]
	if census[2] < minority {
		minority = census[2]
	}
	if minority < 240 {
		t.Fatalf("survivor toss census over 600 seeds = %v: the minority survivor started %d games, "+
			"want >= 240 (uniform p=1/2); a modulo-mapped toss measures ~200", census, minority)
	}
}
