package event_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// ch returns a pointer to seat, for the Entry.Chooser field.
func ch(seat int) *int { return &seat }

// nint returns a pointer to n, for the Entry.Winner field.
func nint(n int) *int { return &n }

// bo3Start logs SeriesStarted Bo3 for the current round's (a, b) series.
func bo3Start(t *testing.T, e *event.Event, round, a, b int) {
	t.Helper()
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: round, A: a, B: b, BestOf: 3})
}

// bo3Lock logs the Sideboarded entries locking both sides for the series'
// next game, each at revision rev.
func bo3Lock(t *testing.T, e *event.Event, round, a, b, game, revA, revB int) {
	t.Helper()
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: round, A: a, B: b, Game: game, Seat: a, Rev: revA})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: round, A: a, B: b, Game: game, Seat: b, Rev: revB})
}

// bo3Game logs one played game of the (a, b) series: a GameStarted attempt
// on its own table (chooser: the play/draw chooser seat, or -1 for a Bo1
// game) followed by its GameEnded (winner: nil for a draw).
func bo3Game(t *testing.T, e *event.Event, round, a, b, game, attempt, chooser int, winner *int) {
	t.Helper()
	table := fmt.Sprintf("t-r%d-s%02d-g%d-a%d", round, a, game, attempt)
	gs := event.Entry{
		Kind: event.KindGameStarted, Round: round, A: a, B: b,
		Game: game, Attempt: attempt, Table: table,
		GameSeed: uint64(1000 + game*10 + attempt),
	}
	if chooser >= 0 {
		gs.Chooser = &chooser
	}
	mustApply(t, e, gs)
	ge := event.Entry{
		Kind: event.KindGameEnded, Round: round, A: a, B: b,
		Game: game, Attempt: attempt,
		Head: fmt.Sprintf("h-r%d-s%02d-g%d-a%d", round, a, game, attempt),
	}
	if winner != nil {
		ge.Winner = winner
	}
	mustApply(t, e, ge)
}

// deckFrom builds a legal 35-card main from the seat's pool, skipping the
// pool card at index skip, plus five basics.
func deckFrom(t *testing.T, e *event.Event, seat, skip int) *build.Deck {
	t.Helper()
	pool := e.Pool(seat)
	if len(pool) < 36 {
		t.Fatalf("seat %d pool has %d cards, want at least 36", seat, len(pool))
	}
	d := build.Deck{Basics: [5]int{5, 0, 0, 0, 0}}
	for i, c := range pool {
		if i == skip {
			continue
		}
		if len(d.Main) == 35 {
			break
		}
		d.Main = append(d.Main, c.ID)
	}
	return &d
}

// TestBo3SeriesEndsAtTwoWins drives a 2–1 and a 2–0 Bo3 series: first to
// two wins decides, a decided series refuses a further game, and
// SeriesEnded is refused before the series is decided or full.
func TestBo3SeriesEndsAtTwoWins(t *testing.T) {
	e := newScriptedEventToRound1(t)
	if e.Phase() != event.RoundPhase(1) {
		t.Fatalf("phase is %s, want ROUND1", e.Phase())
	}
	bo3Start(t, e, 1, 0, 3)
	if err := e.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{0, 0}}); err == nil {
		t.Errorf("SeriesEnded with no games played: want error")
	}

	// Game 1: A wins. Game 2 needs both sides sideboarded first.
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nint(0))
	v, ok := e.SeriesState(0, 3)
	if !ok || !v.Between || v.NextGame != 2 {
		t.Fatalf("after game 1 the series is %+v, want between with next game 2", v)
	}
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 1, Table: "t-g2", Chooser: ch(3)}); err == nil {
		t.Errorf("GameStarted for game 2 before both sides sideboarded: want error")
	}
	bo3Lock(t, e, 1, 0, 3, 2, 1, 1)

	// Game 2: B wins (the game-1 loser chooses and loses). Game 3 needs the
	// locks again — GameEnded cleared them.
	bo3Game(t, e, 1, 0, 3, 2, 1, 3, nint(3))
	v, _ = e.SeriesState(0, 3)
	if !v.Between || v.NextGame != 3 || v.NextChooser != 0 {
		t.Fatalf("after game 2 the series is %+v, want between, next game 3, chooser 0", v)
	}
	bo3Lock(t, e, 1, 0, 3, 3, 1, 1)

	// Game 3: B wins again → 2–1, decided. SeriesEnded accepts it.
	bo3Game(t, e, 1, 0, 3, 3, 1, 0, nint(3))
	v, _ = e.SeriesState(0, 3)
	if v.NextGame != 0 || v.Between {
		t.Fatalf("after game 3 the series is %+v, want decided (NextGame 0)", v)
	}
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 4, Attempt: 1, Table: "t-g4", Chooser: ch(0)}); err == nil {
		t.Errorf("GameStarted for a fourth game of a decided series: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 2}})
	res := e.Results()
	if len(res) == 0 || res[len(res)-1].Score != [2]int{1, 2} || res[len(res)-1].Winner != 3 {
		t.Fatalf("last result = %+v, want 0–3 score [1,2] won by 3", res[len(res)-1])
	}
	if got := res[len(res)-1].Games; len(got) != 3 || got[0] != 0 || got[1] != 3 || got[2] != 3 {
		t.Errorf("last result games = %v, want [0 3 3]", got)
	}

	// A 2–0 series ends after two games: the other round-1 series (1 v 2).
	bo3Start(t, e, 1, 1, 2)
	bo3Game(t, e, 1, 1, 2, 1, 1, 1, nint(1))
	bo3Lock(t, e, 1, 1, 2, 2, 1, 1)
	bo3Game(t, e, 1, 1, 2, 2, 1, 2, nint(1)) // chooser: game 1's loser
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 1, B: 2, Game: 3, Attempt: 1, Table: "t-g3b", Chooser: ch(2)}); err == nil {
		t.Errorf("GameStarted for a third game of a 2–0 series: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 1, B: 2, Score: [2]int{2, 0}})
}

// TestBo3DrawsCapAtThreeGames plays drawn games: a draw never decides, the
// drawn game's chooser chooses again, and three games always fill the
// series (a draw may end it 1–0 or 0–0).
func TestBo3DrawsCapAtThreeGames(t *testing.T) {
	e := newScriptedEventToRound1(t)
	bo3Start(t, e, 1, 0, 3)

	// Game 1 drawn: the chooser is unknown to the draw rule? No: the game
	// had chooser 0, so that chooser chooses again.
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nil)
	v, _ := e.SeriesState(0, 3)
	if !v.Between || v.NextChooser != 0 {
		t.Fatalf("after a drawn game 1 the series is %+v, want between with chooser 0 again", v)
	}
	bo3Lock(t, e, 1, 0, 3, 2, 1, 1)

	// Game 2: A wins. The loser (3) chooses game 3.
	bo3Game(t, e, 1, 0, 3, 2, 1, 0, nint(0))
	bo3Lock(t, e, 1, 0, 3, 3, 1, 1)

	// Game 3 drawn → 1–0, full at three games. A fourth game is refused.
	bo3Game(t, e, 1, 0, 3, 3, 1, 3, nil)
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 4, Attempt: 1, Table: "t-g4", Chooser: ch(3)}); err == nil {
		t.Errorf("GameStarted for a fourth game of a full series: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 1}}); err == nil {
		t.Errorf("SeriesEnded with a score not matching a 1–0 series: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}})
	v, _ = e.SeriesState(0, 3)
	if !v.Ended {
		t.Errorf("series still open after SeriesEnded: %+v", v)
	}

	// draw, draw, draw → 0–0: the other round-1 series (1 v 2).
	bo3Start(t, e, 1, 1, 2)
	bo3Game(t, e, 1, 1, 2, 1, 1, 1, nil)
	bo3Lock(t, e, 1, 1, 2, 2, 1, 1)
	bo3Game(t, e, 1, 1, 2, 2, 1, 1, nil) // the drawn game 1's chooser again
	bo3Lock(t, e, 1, 1, 2, 3, 1, 1)
	bo3Game(t, e, 1, 1, 2, 3, 1, 1, nil)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 1, B: 2, Score: [2]int{0, 0}})
}

// TestBo3ChooserRules pins the play/draw chooser rules: game 1 needs a
// chooser in {A,B}; game 2's must be game 1's loser; after a drawn game the
// previous chooser chooses again; a Bo1 game refuses a chooser; and an old
// Bo1 log (no chooser) still replays.
func TestBo3ChooserRules(t *testing.T) {
	e := newScriptedEventToRound1(t)
	bo3Start(t, e, 1, 0, 3)

	// Game 1: chooser required, and a series seat.
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Table: "t1"}); err == nil {
		t.Errorf("Bo3 game 1 without a chooser: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Table: "t1", Chooser: ch(1)}); err == nil {
		t.Errorf("Bo3 game 1 with a non-series chooser: want error")
	}
	bo3Game(t, e, 1, 0, 3, 1, 1, 3, nint(0)) // seat 3 chooses, loses
	v, _ := e.SeriesState(0, 3)
	if len(v.Choosers) != 1 || v.Choosers[0] != 3 {
		t.Errorf("Choosers after game 1 = %v, want [3]", v.Choosers)
	}
	bo3Lock(t, e, 1, 0, 3, 2, 1, 1)

	// Game 2: the loser of game 1 (seat 3) chooses.
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 1, Table: "t2", Chooser: ch(0)}); err == nil {
		t.Errorf("game 2 chooser = game 1's winner: want error")
	}
	bo3Game(t, e, 1, 0, 3, 2, 1, 3, nil) // seat 3 chooses again, drawn
	bo3Lock(t, e, 1, 0, 3, 3, 1, 1)

	// After a drawn game its chooser (3) must choose again.
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 3, Attempt: 1, Table: "t3", Chooser: ch(0)}); err == nil {
		t.Errorf("game 3 chooser after a drawn game 2 chosen by 3: want error")
	}
	bo3Game(t, e, 1, 0, 3, 3, 1, 3, nint(3))
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 1}})

	// A Bo1 game refuses a chooser.
	b1 := newScriptedEventToRound1(t)
	mustApply(t, b1, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	if err := b1.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Table: "t1", Chooser: ch(0)}); err == nil {
		t.Errorf("Bo1 game with a chooser: want error")
	}
	bo3Game(t, b1, 1, 0, 3, 1, 1, -1, nint(0))
	mustApply(t, b1, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}})

	// An old Bo1 log (no chooser field on any GameStarted) replays.
	wire, err := event.MarshalLog(b1.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	if bytes.Contains(wire, []byte(`"chooser"`)) {
		t.Errorf("a Bo1 log carries a chooser field:\n%s", wire)
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog of an old Bo1 log: %v", err)
	}
	reWire, err := event.MarshalLog(re.Log())
	if err != nil {
		t.Fatalf("re-MarshalLog: %v", err)
	}
	if !bytes.Equal(wire, reWire) {
		t.Errorf("replayed Bo1 log differs from the original")
	}
}

// TestSideboardedRules pins the Sideboarded rules: legal only between
// games, once per side, for a series seat, at a saved revision, for the
// next game; game 2 starts only once both sides are locked; locks survive a
// GameAborted (the retry plays the locked decks) and are cleared by
// GameEnded.
func TestSideboardedRules(t *testing.T) {
	e := newScriptedEventToRound1(t)
	bo3Start(t, e, 1, 0, 3)
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nint(0)) // A wins game 1

	// Refusals while between games.
	sideboarded := event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 0, Rev: 1}
	if err := e.Apply(sideboarded); err != nil {
		t.Fatalf("Sideboarded between games: %v", err)
	}
	if err := e.Apply(sideboarded); err == nil {
		t.Errorf("second Sideboarded for one side: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 1, Rev: 1}); err == nil {
		t.Errorf("Sideboarded for a non-series seat: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 3, Rev: 0}); err == nil {
		t.Errorf("Sideboarded at revision 0: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 3, Rev: 2}); err == nil {
		t.Errorf("Sideboarded beyond the saved revision: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 3, Seat: 3, Rev: 1}); err == nil {
		t.Errorf("Sideboarded for the wrong game: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 3, Rev: 1})
	v, _ := e.SeriesState(0, 3)
	if v.Locked != [2]int{1, 1} {
		t.Fatalf("Locked after both sides sideboarded = %v, want [1 1]", v.Locked)
	}

	// Game 2 starts, is aborted, and the retry plays the locked decks.
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 1, Table: "t-g2", Chooser: ch(3)})
	mustApply(t, e, event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 1, Reason: "crash"})
	v, _ = e.SeriesState(0, 3)
	if v.Locked != [2]int{1, 1} {
		t.Fatalf("Locked after GameAborted = %v, want [1 1] (locks survive)", v.Locked)
	}
	if !v.Between {
		t.Fatalf("series not between games after the abort: %+v", v)
	}
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 2, Table: "t-g2b", Chooser: ch(3)})
	v, _ = e.SeriesState(0, 3)
	if v.PlayRevs != [2]int{1, 1} || v.Locked != [2]int{1, 1} {
		t.Fatalf("retry's PlayRevs = %v, want the locks [1 1] (Locked %v)", v.PlayRevs, v.Locked)
	}
	if v.InFlight == nil || v.InFlight.Game != 2 || v.InFlight.Attempt != 2 {
		t.Fatalf("InFlight after the retry = %+v, want game 2 attempt 2", v.InFlight)
	}

	// Game 2 ends (A wins → decided): the locks clear.
	ge := event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 2, Attempt: 2, Head: "h-g2"}
	ge.Winner = nint(0)
	mustApply(t, e, ge)
	v, _ = e.SeriesState(0, 3)
	if v.Locked != [2]int{0, 0} {
		t.Fatalf("Locked after GameEnded = %v, want [0 0]", v.Locked)
	}
	if v.Between || v.NextGame != 0 {
		t.Fatalf("series after a 2–0 decision = %+v, want decided", v)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{2, 0}})
}

// newScriptedEventInBuild runs the scripted prelude and auto-picks every
// pack to completion, leaving the event in BUILD with no DeckSaved applied.
func newScriptedEventInBuild(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			progressed = true
			mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
		}
		if !progressed {
			t.Fatalf("draft stalled in phase %s", e.Phase())
		}
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("fully picked event in phase %s, want BUILD", e.Phase())
	}
	return e
}

// TestDeckSavedBetweenGames checks the between-games deck rules: a round
// phase DeckSaved is legal for an unlocked side of a between-games series
// and does not reset its readiness, is refused once the side is locked or
// the series is live or over, and the BUILD-phase behaviour is unchanged.
func TestDeckSavedBetweenGames(t *testing.T) {
	e := newScriptedEventToRound1(t)
	if got := e.ReadyRev(0); got != 1 {
		t.Fatalf("ReadyRev(0) after the prelude = %d, want 1", got)
	}
	bo3Start(t, e, 1, 0, 3)
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nint(0))

	// Between games: seat 0 saves revision 2; it must be readied again
	// never (a round-phase save does not touch readiness) and the lock
	// still accepts it.
	d2 := deckFrom(t, e, 0, 1)
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: d2})
	if got := e.ReadyRev(0); got != 1 {
		t.Errorf("ReadyRev(0) after a round-phase DeckSaved = %d, want 1 (round saves never reset readiness)", got)
	}
	if got := e.SavedRev(0); got != 2 {
		t.Errorf("SavedRev(0) = %d, want 2", got)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 0, Rev: 2})
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 3, Deck: deckFrom(t, e, 0, 2)}); err == nil {
		t.Errorf("DeckSaved for a locked side: want error")
	}

	// A live (in-flight) Bo1 series: DeckSaved is refused, and stays
	// refused once the series is over (Bo1 is never between games).
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 1, B: 2, BestOf: 1})
	bo3Game(t, e, 1, 1, 2, 1, 1, -1, nint(1))
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 1, Rev: 2, Deck: deckFrom(t, e, 1, 1)}); err == nil {
		t.Errorf("DeckSaved while the seat's series game is in flight: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 1, B: 2, Score: [2]int{1, 0}})
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 1, Rev: 2, Deck: deckFrom(t, e, 1, 1)}); err == nil {
		t.Errorf("DeckSaved after the seat's series ended: want error")
	}

	// BUILD behaviour unchanged: a fresh BUILD-phase save clears readiness.
	b := newScriptedEventInBuild(t)
	if b.Phase() != event.PhaseBuild {
		t.Fatalf("mid-draft event in phase %s, want BUILD", b.Phase())
	}
	mustApply(t, b, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: deckFrom(t, b, 0, -1)})
	mustApply(t, b, event.Entry{Kind: event.KindReady, Seat: 0, Rev: 1})
	mustApply(t, b, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, b, 0, 1)})
	if got := b.ReadyRev(0); got != 0 {
		t.Errorf("ReadyRev(0) after a BUILD-phase DeckSaved = %d, want 0", got)
	}
}

// TestSeriesStateIsReadOnly proves SeriesState/SeriesOf never create series
// state and never touch the derived state, and that a view's slices are
// copies.
func TestSeriesStateIsReadOnly(t *testing.T) {
	e := newScriptedEventToRound1(t)
	before, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}

	v, ok := e.SeriesState(0, 3)
	if !ok {
		t.Fatal("SeriesState(0,3) = false for a round-1 pairing")
	}
	if v.Started || v.Ended || v.Between || v.BestOf != 0 || v.NextGame != 1 || v.NextChooser != -1 {
		t.Errorf("fresh pairing's view = %+v, want untouched: Started false, NextGame 1, NextChooser -1", v)
	}
	if len(v.Games) != 0 || len(v.Choosers) != 0 || v.Wins != [2]int{} || v.InFlight != nil {
		t.Errorf("fresh pairing's view carries state: %+v", v)
	}
	if _, ok := e.SeriesState(0, 1); ok {
		t.Errorf("SeriesState(0,1) = true for a non-pairing")
	}
	if _, ok := e.SeriesState(3, 0); ok {
		t.Errorf("SeriesState(3,0) = true for a reversed pairing")
	}
	for seat, want := range map[int][2]int{0: {0, 3}, 1: {1, 2}, 2: {1, 2}, 3: {0, 3}} {
		a, b, ok := e.SeriesOf(seat)
		if !ok || a != want[0] || b != want[1] {
			t.Errorf("SeriesOf(%d) = (%d,%d,%v), want (%d,%d,true)", seat, a, b, ok, want[0], want[1])
		}
	}
	if _, _, ok := e.SeriesOf(9); ok {
		t.Errorf("SeriesOf(9) = ok for an out-of-range seat")
	}
	if _, _, ok := e.SeriesOf(-1); ok {
		t.Errorf("SeriesOf(-1) = ok")
	}

	after, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON after the accessors: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("StateJSON changed under the read-only accessors")
	}
	if err := e.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 3}); err != nil {
		t.Fatalf("SeriesStarted after the accessors: %v (the accessors must not have created series state)", err)
	}

	// The view's slices are copies: mutating one leaves the state alone.
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nint(0))
	v, _ = e.SeriesState(0, 3)
	if len(v.Games) != 1 || v.Games[0] != 0 {
		t.Fatalf("Games after game 1 = %v, want [0]", v.Games)
	}
	v.Games[0] = 7
	v.Games = append(v.Games, 8)
	v2, _ := e.SeriesState(0, 3)
	if len(v2.Games) != 1 || v2.Games[0] != 0 {
		t.Errorf("the view's Games slice aliases the state: %v", v2.Games)
	}
}

// TestStandingsCountBo3Games checks the standings over Bo3 results: each
// seat's GW% equals game points / (3 × games played), floored at 0.33, and
// MW% comes from the series points.
func TestStandingsCountBo3Games(t *testing.T) {
	rs := []event.SeriesResult{
		// Round 1: 0 beats 1 2–1; 2 beats 3 2–0.
		{Round: 1, A: 0, B: 1, Score: [2]int{2, 1}, Games: []event.GameOutcome{event.AWon, event.BWon, event.AWon}},
		{Round: 1, A: 2, B: 3, Score: [2]int{2, 0}, Games: []event.GameOutcome{event.AWon, event.AWon}},
		// Round 2: 0 and 2 draw 1–1 (win, draw, loss); 1 beats 3 2–0.
		{Round: 2, A: 0, B: 2, Score: [2]int{1, 1}, Games: []event.GameOutcome{event.AWon, event.Draw, event.BWon}},
		{Round: 2, A: 1, B: 3, Score: [2]int{2, 0}, Games: []event.GameOutcome{event.AWon, event.AWon}},
	}
	st := event.ComputeStandings(rs)
	want := map[int][4]float64{
		// seat: MW, OMW, GW, OGW
		0: {4.0 / 6, (0.5 + 2.0/3) / 2, 10.0 / 18, (0.6 + 2.0/3) / 2},
		1: {3.0 / 6, (2.0/3 + floor33) / 2, 9.0 / 15, (10.0/18 + floor33) / 2},
		2: {4.0 / 6, (2.0/3 + floor33) / 2, 10.0 / 15, (10.0/18 + floor33) / 2},
		3: {floor33, (0.5 + 2.0/3) / 2, floor33, (0.6 + 2.0/3) / 2},
	}
	points := map[int]int{0: 4, 1: 3, 2: 4, 3: 0}
	for _, s := range st {
		w := want[s.Seat]
		if s.Points != points[s.Seat] {
			t.Errorf("seat %d points = %d, want %d", s.Seat, s.Points, points[s.Seat])
		}
		for i, got := range []float64{s.MW, s.OMW, s.GW, s.OGW} {
			if math.Abs(got-w[i]) > 1e-9 {
				t.Errorf("seat %d column %d = %.6f, want %.6f", s.Seat, i, got, w[i])
			}
		}
	}
	if len(st) != 4 {
		t.Fatalf("standings table has %d rows, want 4", len(st))
	}
}

// floor33 is MTR Appendix C's 33% floor, spelled out for the expectations.
const floor33 = 0.33

// TestParseLogAcceptsSideboarded proves the Sideboarded kind parses, round-trips
// through MarshalLog/ReplayLog byte-identically, and that a round-phase
// DeckSaved deck stays hidden from other seats before COMPLETE.
func TestParseLogAcceptsSideboarded(t *testing.T) {
	e := newScriptedEventToRound1(t)
	bo3Start(t, e, 1, 0, 3)
	bo3Game(t, e, 1, 0, 3, 1, 1, 0, nint(0))
	// The human seat 0 swaps a card, then both sides lock (seat 3's lock is
	// the server keeping the deck for it: Auto).
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, e, 0, 1)})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 3, Rev: 1, Auto: true})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 1, A: 0, B: 3, Game: 2, Seat: 0, Rev: 2})
	bo3Game(t, e, 1, 0, 3, 2, 1, 3, nint(3))

	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	if !bytes.Contains(wire, []byte(`"kind":"Sideboarded"`)) {
		t.Errorf("the log lacks a Sideboarded line:\n%s", wire)
	}
	if !bytes.Contains(wire, []byte(`"chooser":3`)) {
		t.Errorf("the log lacks a chooser field:\n%s", wire)
	}
	parsed, err := event.ParseLog(wire)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	sawSideboarded := false
	for _, en := range parsed {
		if en.Kind != event.KindSideboarded {
			continue
		}
		sawSideboarded = true
		if en.Round != 1 || en.A != 0 || en.B != 3 || en.Game != 2 || (en.Seat != 0 && en.Seat != 3) || en.Rev != 2 && en.Seat == 0 {
			t.Errorf("parsed Sideboarded = %+v, want round 1, 0–3, game 2, a series seat", en)
		}
	}
	if !sawSideboarded {
		t.Fatal("ParseLog dropped every Sideboarded entry")
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	reWire, err := event.MarshalLog(re.Log())
	if err != nil {
		t.Fatalf("re-MarshalLog: %v", err)
	}
	if !bytes.Equal(wire, reWire) {
		t.Errorf("replayed log differs from the original:\n%s\nvs\n%s", wire, reWire)
	}

	// Before COMPLETE the round-phase DeckSaved deck is the seat's private
	// information; the Sideboarded lines are public.
	for viewer, visible := range map[int]bool{0: true, 1: false, 3: false, -1: false} {
		saw := false
		for _, en := range re.LogFor(viewer) {
			if en.Kind != event.KindDeckSaved || en.Seat != 0 || en.Rev != 2 {
				continue
			}
			saw = true
			if visible && (en.Deck == nil || len(en.Deck.Main) == 0) {
				t.Errorf("LogFor(%d) hid seat 0's own round-phase deck", viewer)
			}
			if !visible && en.Deck != nil {
				t.Errorf("LogFor(%d) leaked seat 0's round-phase deck", viewer)
			}
		}
		if !saw {
			t.Errorf("LogFor(%d) has no round-phase DeckSaved entry to check", viewer)
		}
	}

	// GameAborted keeps the locks, and a replayed log reproduces the state:
	// SeriesState on the replayed event matches the original's (after a
	// 1–1 game 2 the series is between games, chooser 0 picks again).
	for _, ev := range []*event.Event{e, re} {
		v, ok := ev.SeriesState(0, 3)
		if !ok {
			t.Fatalf("SeriesState(0,3) = false after game 2")
		}
		if v.Ended || !v.Between || v.Wins != [2]int{1, 1} || v.NextGame != 3 || v.NextChooser != 0 ||
			len(v.Choosers) != 2 || v.Choosers[0] != 0 || v.Choosers[1] != 3 {
			t.Errorf("series view after game 2 = %+v, want between games 1–1, chooser 0 next", v)
		}
	}
}
