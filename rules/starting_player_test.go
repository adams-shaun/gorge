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
		if ev.Kind == events.Note && ev.Counter == events.NoteToss {
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
// public -- exactly one Note per game carrying the "won the toss" claim,
// emitted BEFORE the first shuffle (CR 103.1 precedes 103.2-103.4), carrying
// the winner as its Player; who actually takes the first turn is the Toss
// event, folded into g.Active.
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
		want := tossName(e.G, e.G.Active) + " won the toss"
		if notes[0].Text != want {
			t.Fatalf("seed=%d: toss Note text %q, want %q", seed, notes[0].Text, want)
		}
		// The text cannot claim the first turn before the deal has fixed the
		// survivors; the Toss event is the one that carries that resolution.
		for _, ev := range e.L.Events {
			if ev.Kind == events.Toss && ev.Player != e.G.Active {
				t.Fatalf("seed=%d: Toss event names seat %d, game started at %d", seed, ev.Player, e.G.Active)
			}
		}
	}
}

// TestTossNotePrecedesTheFirstShuffle is the CR 103.1 ordering gate (fix
// round rv2a): the toss announcement must be public before the decks are
// shuffled and the opening hands dealt -- Forge's GameAction and manabrew's
// game loop announce the toss before their deal, and the whole point is that
// the keep/mulligan decisions are made with the toss already read. The old
// engine emitted the Note after the last opening draw.
func TestTossNotePrecedesTheFirstShuffle(t *testing.T) {
	for _, mulligans := range []int{0, 1} {
		cfg := tossedTwoSeat(t, 1, mulligans)
		e := New(cfg)
		noteAt, shuffleAt := -1, -1
		for i, ev := range e.L.Events {
			switch {
			case ev.Kind == events.Note && ev.Counter == events.NoteToss && noteAt < 0:
				noteAt = i
			case ev.Kind == events.Shuffle && shuffleAt < 0:
				shuffleAt = i
			}
		}
		if noteAt < 0 || shuffleAt < 0 {
			t.Fatalf("mulligans=%d: toss Note at %d, first Shuffle at %d", mulligans, noteAt, shuffleAt)
		}
		if noteAt > shuffleAt {
			t.Fatalf("mulligans=%d: toss Note (event %d) came after the first Shuffle (event %d)", mulligans, noteAt, shuffleAt)
		}
		// And the Toss resolution event comes after the deal (it needs the
		// survivors) but before the first mulligan ask or the first turn.
		tossAt := -1
		for i, ev := range e.L.Events {
			if ev.Kind == events.Toss {
				tossAt = i
				break
			}
		}
		if tossAt < 0 || tossAt < shuffleAt {
			t.Fatalf("mulligans=%d: Toss event at %d, want one after the first Shuffle (%d)", mulligans, tossAt, shuffleAt)
		}
	}
}

// TestTossNoteIsEmittedWhenOpeningDealEndsTheGame covers either seat losing
// during New's deal loop. The pre-deal announcement still names the CR 103.1
// toss winner even when the deal eliminated them (the toss DID happen, and
// truthfully, before the deal); the resolution Note records only why no turn
// began, and GameOver remains the burst's final event for host persistence.
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
		// The pre-deal announcement names the toss winner -- the seat the rng
		// handed the toss to, whatever the deal did to them (seed 1 tosses to
		// seat 1, measured in TestTossedGameReplaysByteIdentically).
		if got, want := notes[0].Text, tossName(e.G, notes[0].Player)+" won the toss"; got != want {
			t.Fatalf("short seat %d: toss Note text %q, want %q", shortSeat, got, want)
		}
		// No Toss resolution event in a game that never began a turn, and a
		// resolution Note explaining why.
		for _, ev := range e.L.Events {
			if ev.Kind == events.Toss {
				t.Fatalf("short seat %d: terminal genesis emitted a Toss event", shortSeat)
			}
		}
		resolutions := 0
		for _, ev := range e.L.Events {
			if ev.Kind == events.Note && ev.Text == "The game ended before the first turn" {
				resolutions++
			}
		}
		if resolutions != 1 {
			t.Fatalf("short seat %d: %d resolution Notes, want exactly 1", shortSeat, resolutions)
		}
		if got := e.L.Events[len(e.L.Events)-1].Kind; got != events.GameOver {
			t.Fatalf("short seat %d: final genesis event = %v, want GameOver", shortSeat, got)
		}
	}
}

// TestTossNoteSurvivesWhenEveryOpeningDeckIsUndersized pins the no-survivor
// terminal shape: the pre-drawn random determination is still recorded (the
// pre-deal announcement, before any shuffle) even though nobody remains to
// become starting player, and the resolution Note precedes the CR 104.4a
// draw, leaving GameOver as the final event.
func TestTossNoteSurvivesWhenEveryOpeningDeckIsUndersized(t *testing.T) {
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 3), mountainDeck(t, 3)}})
	if !e.G.Over || !e.G.Draw || e.G.Turn != 0 || e.G.AliveCount() != 0 {
		t.Fatalf("terminal genesis = over %v draw %v turn %d alive %d, want a before-turn draw with no survivors",
			e.G.Over, e.G.Draw, e.G.Turn, e.G.AliveCount())
	}
	notes := tossNotes(e)
	if len(notes) != 1 {
		t.Fatalf("toss Notes = %d, want exactly 1", len(notes))
	}
	want := tossName(e.G, notes[0].Player) + " won the toss"
	if notes[0].Text != want {
		t.Fatalf("toss Note text %q, want %q", notes[0].Text, want)
	}
	if len(e.L.Events) < 2 || e.L.Events[len(e.L.Events)-2].Kind != events.Note || e.L.Events[len(e.L.Events)-1].Kind != events.GameOver {
		t.Fatalf("terminal genesis tail = %+v, want resolution Note then GameOver", e.L.Events[max(0, len(e.L.Events)-2):])
	}
	if got := e.L.Events[len(e.L.Events)-2].Text; got != "The game ended before the first turn" {
		t.Fatalf("terminal resolution Note = %q", got)
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
	// The Toss event has folded the toss winner into g.Active already, so
	// the pregame projection (view) reads the real starting seat; the round
	// order is what New built from it. The toss Note names the winner --
	// that is the fixture's precondition.
	notes := tossNotes(e)
	if len(notes) != 1 || notes[0].Player != 1 {
		t.Fatalf("fixture precondition failed: toss notes %+v", notes)
	}
	if e.G.Active != 1 {
		t.Fatalf("pregame g.Active = %d, want the toss winner (seat 1) via the Toss event", e.G.Active)
	}
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KMulligan {
		t.Fatalf("pending = %+v, want the first keep/mulligan ask", d)
	}
	if d.Player != 1 {
		t.Fatalf("first mulligan ask went to seat %d, want the toss winner (seat 1) -- CR 103.5", d.Player)
	}
	// The prompt names who plays first: the ask is the one always-visible
	// surface a seated human reads before choosing (the transcript starts
	// hidden), so it must carry the play/draw fact itself.
	if !strings.Contains(d.Prompt, tossName(e.G, 1)+" plays first") {
		t.Fatalf("mulligan prompt %q does not name the starting player", d.Prompt)
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

// TestMulliganPromptNamesTheDisplayPlayerWithDuplicateDecks is the
// seat-facing integration contract: duplicate deck identities are valid, so
// only the table's PlayerName tells a human whether they play or draw. It
// drives rules.New through its real Toss and mulligan ask rather than
// handcrafting the prompt the web receives.
func TestMulliganPromptNamesTheDisplayPlayerWithDuplicateDecks(t *testing.T) {
	cfg := tossedTwoSeat(t, 1, 1) // seed 1 starts seat 1
	cfg.Names = []string{"same-deck", "same-deck"}
	cfg.PlayerNames = []string{"Alice", "Bob"}
	e := New(cfg)
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KMulligan || d.Player != 1 {
		t.Fatalf("pending = %+v, want seat 1 mulligan", d)
	}
	if got, want := d.Prompt, "Bob plays first. Keep your hand (put 0 cards on the bottom of your library) or take a mulligan?"; got != want {
		t.Fatalf("duplicate-deck mulligan prompt = %q, want %q", got, want)
	}
	if strings.Contains(d.Prompt, "same-deck plays first") {
		t.Fatalf("duplicate-deck mulligan prompt is ambiguous: %q", d.Prompt)
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
