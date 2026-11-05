package event_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// newScriptedRebuildEvent runs the scripted prelude (see
// newScriptedEventToRound1) with the EventCreated entry carrying
// Rebuild: true, leaving the event in ROUND1 with every seat ready.
func newScriptedRebuildEvent(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: testSeats(), Rebuild: true})
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
	for s := 0; s < draft.Seats; s++ {
		pool := e.Pool(s)
		d := build.Deck{Main: make([]string, 0, 35), Basics: [5]int{5, 0, 0, 0, 0}}
		for _, c := range pool[:35] {
			d.Main = append(d.Main, c.ID)
		}
		mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d})
		mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
	return e
}

// playRound1Bo1 plays both round-1 series as Bo1, seat A winning each, and
// ends them, leaving the event in ROUND2.
func playRound1Bo1(t *testing.T, e *event.Event) {
	t.Helper()
	for _, p := range [][2]int{{0, 3}, {1, 2}} {
		mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: p[0], B: p[1], BestOf: 1})
		bo3Game(t, e, 1, p[0], p[1], 1, 1, -1, nint(p[0]))
		mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: p[0], B: p[1], Score: [2]int{1, 0}})
	}
	if e.Phase() != event.RoundPhase(2) {
		t.Fatalf("phase after round 1 = %s, want ROUND2", e.Phase())
	}
}

// newRebuildEventToRound2 returns a Rebuild event in ROUND2, round 1 played.
func newRebuildEventToRound2(t *testing.T) *event.Event {
	t.Helper()
	e := newScriptedRebuildEvent(t)
	playRound1Bo1(t, e)
	return e
}

// TestRebuildRoundOneUsesReadiedDecks pins round 1 unchanged under Rebuild:
// game 1 needs no locks, Opening is false, and PlayRevs are the readied
// revisions.
func TestRebuildRoundOneUsesReadiedDecks(t *testing.T) {
	e := newScriptedRebuildEvent(t)
	if !e.Rebuild() {
		t.Fatal("Rebuild() = false after an EventCreated with Rebuild: true")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	v, ok := e.SeriesState(0, 3)
	if !ok || !v.Started {
		t.Fatalf("SeriesState(0,3) = (%+v,%v), want a started series", v, ok)
	}
	if v.Opening {
		t.Errorf("round-1 series reports Opening: %+v", v)
	}
	// Game 1 starts with no locks and plays the readied decks.
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Table: "t-r1"})
	v, _ = e.SeriesState(0, 3)
	if v.PlayRevs != [2]int{e.ReadyRev(0), e.ReadyRev(3)} {
		t.Errorf("round-1 PlayRevs = %v, want the readied revisions (%d, %d)",
			v.PlayRevs, e.ReadyRev(0), e.ReadyRev(3))
	}
	if e.LastPlayedRev(0) != e.ReadyRev(0) {
		t.Errorf("LastPlayedRev(0) = %d, want the readied revision %d", e.LastPlayedRev(0), e.ReadyRev(0))
	}
	ge := event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Head: "h-r1"}
	ge.Winner = nint(0)
	mustApply(t, e, ge)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}})
}

// TestRebuildWindowOpensFromRoundTwo drives a round-2 rebuild window: the
// series opens, game 1 is refused unlocked, seat A saves and locks a new
// revision while seat B locks its keep, and game 1 then plays the locked
// revisions.
func TestRebuildWindowOpensFromRoundTwo(t *testing.T) {
	e := newRebuildEventToRound2(t)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1})
	v, _ := e.SeriesState(0, 2)
	if !v.Opening || v.Between {
		t.Fatalf("round-2 series before its first game = %+v, want Opening", v)
	}
	// Game 1 is refused until both sides are locked.
	err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-r2"})
	if err == nil || !strings.Contains(err.Error(), "MTR 7.3") {
		t.Fatalf("unlocked round-2 game 1 error = %v, want the MTR 7.3 refusal", err)
	}
	// Seat 0 saves a new revision in the window and locks it; seat 2 locks
	// its keep (the revision it played last round).
	d2 := deckFrom(t, e, 0, 1)
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: d2})
	keepB := e.LastPlayedRev(2)
	if keepB != 1 {
		t.Fatalf("LastPlayedRev(2) in the round-2 window = %d, want 1", keepB)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 0, Rev: 2})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 2, Rev: keepB})
	v, _ = e.SeriesState(0, 2)
	if !v.Opening || v.Locked != [2]int{2, keepB} {
		t.Fatalf("locked round-2 series = %+v, want still opening (the window closes when game 1 starts) with locks [2 %d]", v, keepB)
	}
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-r2"})
	v, _ = e.SeriesState(0, 2)
	if v.PlayRevs != [2]int{2, keepB} {
		t.Errorf("round-2 game-1 PlayRevs = %v, want [2 %d]", v.PlayRevs, keepB)
	}
	if got := e.LastPlayedRev(0); got != 2 {
		t.Errorf("LastPlayedRev(0) after game 1 started = %d, want 2", got)
	}
}

// TestRebuildKeepCarriesAcrossRounds checks that a revision locked and
// played in round 2 is the keep revision of round 3's window.
func TestRebuildKeepCarriesAcrossRounds(t *testing.T) {
	e := newRebuildEventToRound2(t)
	// Round 2: seat 0 rebuilds to revision 2 and wins; seat 2 keeps 1 and
	// loses. The other series (1, 3) plays with keeps on both sides.
	for _, p := range [][2]int{{0, 2}, {1, 3}} {
		mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: p[0], B: p[1], BestOf: 1})
		keepA, keepB := e.LastPlayedRev(p[0]), e.LastPlayedRev(p[1])
		revA := keepA
		if p[0] == 0 {
			mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, e, 0, 1)})
			revA = 2
		}
		mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: p[0], B: p[1], Game: 1, Seat: p[0], Rev: revA})
		mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: p[0], B: p[1], Game: 1, Seat: p[1], Rev: keepB})
		mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: p[0], B: p[1], Game: 1, Attempt: 1, Table: "t-r2"})
		ge := event.Entry{Kind: event.KindGameEnded, Round: 2, A: p[0], B: p[1], Game: 1, Attempt: 1, Head: "h-r2"}
		ge.Winner = nint(p[0])
		mustApply(t, e, ge)
		mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 2, A: p[0], B: p[1], Score: [2]int{1, 0}})
	}
	if e.Phase() != event.RoundPhase(3) {
		t.Fatalf("phase after round 2 = %s, want ROUND3", e.Phase())
	}
	// Round 3: the keep revision of each seat is the one it last played.
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 3, A: 0, B: 1, BestOf: 1})
	if got := e.LastPlayedRev(0); got != 2 {
		t.Errorf("LastPlayedRev(0) in the round-3 window = %d, want 2", got)
	}
	if got := e.LastPlayedRev(1); got != 1 {
		t.Errorf("LastPlayedRev(1) in the round-3 window = %d, want 1", got)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 3, A: 0, B: 1, Game: 1, Seat: 0, Rev: e.LastPlayedRev(0)})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 3, A: 0, B: 1, Game: 1, Seat: 1, Rev: e.LastPlayedRev(1)})
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 3, A: 0, B: 1, Game: 1, Attempt: 1, Table: "t-r3"})
	v, _ := e.SeriesState(0, 1)
	if v.PlayRevs != [2]int{2, 1} {
		t.Errorf("round-3 PlayRevs = %v, want [2 1] (the carried keeps)", v.PlayRevs)
	}
}

// TestRebuildOffUnchanged pins the Rebuild: false path: a round-2 game 1
// starts with no locks and plays the readied decks, and a round-2 DeckSaved
// before game 1 is refused.
func TestRebuildOffUnchanged(t *testing.T) {
	e := newScriptedEventToRound1(t)
	playRound1Bo1(t, e)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1})
	v, _ := e.SeriesState(0, 2)
	if v.Opening {
		t.Errorf("a Rebuild: false series reports Opening: %+v", v)
	}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, e, 0, 1)}); err == nil {
		t.Errorf("round-2 DeckSaved before game 1 with Rebuild off: want error")
	}
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-r2"})
	v, _ = e.SeriesState(0, 2)
	if v.PlayRevs != [2]int{e.ReadyRev(0), e.ReadyRev(2)} {
		t.Errorf("Rebuild-off round-2 PlayRevs = %v, want the readied revisions", v.PlayRevs)
	}
}

// TestRebuildConcessionNeedsNoLocks checks the concession path: a GameEnded
// for a not-yet-attempted game 1 of an opening series is accepted without
// any locks.
func TestRebuildConcessionNeedsNoLocks(t *testing.T) {
	e := newRebuildEventToRound2(t)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1})
	ge := event.Entry{Kind: event.KindGameEnded, Round: 2, A: 0, B: 2, Game: 1, Head: "conceded"}
	ge.Winner = nint(0)
	mustApply(t, e, ge)
	v, _ := e.SeriesState(0, 2)
	if !v.Started || v.Opening || len(v.Games) != 1 || v.Games[0] != 0 {
		t.Fatalf("series after the concession = %+v, want one game won by 0", v)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 2, A: 0, B: 2, Score: [2]int{1, 0}})
}

// TestRebuildBo3LaterGamesUnchanged pins the Bo3 series of a rebuild event:
// game 1 locks in the window, then games 2 and 3 sideboard exactly as
// before, and Opening is false after game 1.
func TestRebuildBo3LaterGamesUnchanged(t *testing.T) {
	e := newRebuildEventToRound2(t)
	bo3Start(t, e, 2, 0, 2)
	v, _ := e.SeriesState(0, 2)
	if !v.Opening {
		t.Fatalf("round-2 Bo3 series = %+v, want Opening", v)
	}
	bo3Lock(t, e, 2, 0, 2, 1, 1, 1)
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-g1", Chooser: ch(0)})
	v, _ = e.SeriesState(0, 2)
	if v.Opening {
		t.Errorf("series after game 1 started still reports Opening: %+v", v)
	}
	// Game 1: A wins. Game 2 needs both sides sideboarded first.
	ge := event.Entry{Kind: event.KindGameEnded, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Head: "h-g1"}
	ge.Winner = nint(0)
	mustApply(t, e, ge)
	v, _ = e.SeriesState(0, 2)
	if v.Opening || !v.Between || v.NextGame != 2 {
		t.Fatalf("series after game 1 = %+v, want between with next game 2", v)
	}
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 2, Attempt: 1, Table: "t-g2", Chooser: ch(2)}); err == nil {
		t.Errorf("game 2 before both sides sideboarded: want error")
	}
	bo3Lock(t, e, 2, 0, 2, 2, 1, 1)
	// Game 2: B wins (the game-1 loser chooses and loses). Game 3 needs the
	// locks again.
	bo3Game(t, e, 2, 0, 2, 2, 1, 2, nint(2))
	bo3Lock(t, e, 2, 0, 2, 3, 1, 1)
	bo3Game(t, e, 2, 0, 2, 3, 1, 0, nint(2)) // 1–2, decided; game 2's loser chooses
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 2, A: 0, B: 2, Score: [2]int{1, 2}})
}

// TestRebuildAbortKeepsLocks checks that a GameAborted game-1 attempt keeps
// the window's locks and the retry plays the same revisions.
func TestRebuildAbortKeepsLocks(t *testing.T) {
	e := newRebuildEventToRound2(t)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1})
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, e, 0, 1)})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 0, Rev: 2})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 2, Rev: 1})
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-a1"})
	mustApply(t, e, event.Entry{Kind: event.KindGameAborted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Reason: "crash"})
	v, _ := e.SeriesState(0, 2)
	if !v.Opening || v.Locked != [2]int{2, 1} {
		t.Fatalf("series after the abort = %+v, want Opening with locks [2 1]", v)
	}
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 2, Table: "t-a2"})
	v, _ = e.SeriesState(0, 2)
	if v.PlayRevs != [2]int{2, 1} {
		t.Errorf("retry's PlayRevs = %v, want the same locks [2 1]", v.PlayRevs)
	}
}

// TestRebuildStateJSONKey checks the snapshot's rebuild key and that a
// rebuild log round-trips through MarshalLog/ReplayLog to the same
// StateJSON.
func TestRebuildStateJSONKey(t *testing.T) {
	plain := newScriptedEventToRound1(t)
	plainJSON, err := plain.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	if bytes.Contains(plainJSON, []byte(`"rebuild"`)) {
		t.Errorf("a Rebuild: false event's snapshot carries a rebuild key:\n%s", plainJSON)
	}
	var shape map[string]any
	if err := json.Unmarshal(plainJSON, &shape); err != nil {
		t.Fatalf("unmarshal plain snapshot: %v", err)
	}
	if _, has := shape["rebuild"]; has {
		t.Errorf("a Rebuild: false event's snapshot has a rebuild key")
	}

	e := newRebuildEventToRound2(t)
	rebuildJSON, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	if !bytes.Contains(rebuildJSON, []byte(`"rebuild":true`)) {
		t.Errorf("a rebuild event's snapshot lacks \"rebuild\":true:\n%s", rebuildJSON)
	}

	// Drive the round-2 window, then marshal and replay the log.
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1})
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: deckFrom(t, e, 0, 1)})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 0, Rev: 2})
	mustApply(t, e, event.Entry{Kind: event.KindSideboarded, Round: 2, A: 0, B: 2, Game: 1, Seat: 2, Rev: 1})
	mustApply(t, e, event.Entry{Kind: event.KindGameStarted, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Table: "t-r2"})
	ge := event.Entry{Kind: event.KindGameEnded, Round: 2, A: 0, B: 2, Game: 1, Attempt: 1, Head: "h-r2"}
	ge.Winner = nint(0)
	mustApply(t, e, ge)

	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	orig, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON of the original: %v", err)
	}
	rep, err := re.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON of the replay: %v", err)
	}
	if !bytes.Equal(orig, rep) {
		t.Errorf("the replayed rebuild event's StateJSON differs from the original")
	}
}
