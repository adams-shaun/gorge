package eventhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// bo3Series is one pairing's Bo3 entries in log order, grouped by kind.
type bo3Series struct {
	started  event.Entry
	starts   []event.Entry // GameStarted, log order
	aborts   []event.Entry // GameAborted, log order
	ends     []event.Entry // GameEnded, log order
	sideb    []event.Entry // Sideboarded, log order
	finished event.Entry   // SeriesEnded
}

// groupSeries splits a log's series entries per round and pairing,
// preserving order (the same two seats can meet again in a later round).
func groupSeries(log []event.Entry) map[[3]int]*bo3Series {
	out := map[[3]int]*bo3Series{}
	for _, en := range log {
		var key [3]int
		switch en.Kind {
		case event.KindSeriesStarted:
			key = [3]int{en.Round, en.A, en.B}
		case event.KindGameStarted, event.KindGameAborted, event.KindGameEnded, event.KindSideboarded, event.KindSeriesEnded:
			key = [3]int{en.Round, en.A, en.B}
		default:
			continue
		}
		s := out[key]
		if s == nil {
			s = &bo3Series{}
			out[key] = s
		}
		switch en.Kind {
		case event.KindSeriesStarted:
			s.started = en
		case event.KindGameStarted:
			s.starts = append(s.starts, en)
		case event.KindGameAborted:
			s.aborts = append(s.aborts, en)
		case event.KindGameEnded:
			s.ends = append(s.ends, en)
		case event.KindSideboarded:
			s.sideb = append(s.sideb, en)
		case event.KindSeriesEnded:
			s.finished = en
		}
	}
	return out
}

// checkChooserPlan asserts one GameStarted's chooser against the series
// rules: game 1 the table's toss winner (the log's own GameSeed), later
// games MTR 2.2's chooser over the previous ENDED game (its winner, or its
// chooser after a draw).
func checkChooserPlan(t *testing.T, pair [3]int, en event.Entry, prevStart, prevEnd *event.Entry) {
	t.Helper()
	if en.Chooser == nil {
		t.Fatalf("series %v game %d attempt %d has no chooser", pair, en.Game, en.Attempt)
	}
	c := *en.Chooser
	if c != en.A && c != en.B {
		t.Fatalf("series %v game %d chooser %d is not a series seat", pair, en.Game, c)
	}
	if en.Game == 1 {
		toss := host.MatchTossWinner(en.GameSeed, 1, 2)
		want := en.A
		if toss == 1 {
			want = en.B
		}
		if c != want {
			t.Fatalf("series %v game 1 chooser %d, want toss winner %d", pair, c, want)
		}
		return
	}
	want := prevStart.Chooser
	if prevEnd.Winner != nil {
		w := en.A
		if *prevEnd.Winner == en.A {
			w = en.B
		}
		want = &w
	}
	if want == nil || c != *want {
		t.Fatalf("series %v game %d chooser %d, want %v (MTR 2.2)", pair, en.Game, c, want)
	}
}

// checkBo3Shape asserts one played-out (no crash) Bo3 series' log shape and
// returns its SeriesEnded score.
func checkBo3Shape(t *testing.T, pair [3]int, s *bo3Series) [2]int {
	t.Helper()
	if s.started.Kind != event.KindSeriesStarted || s.started.BestOf != 3 {
		t.Fatalf("series %v SeriesStarted = %+v", pair, s.started)
	}
	n := len(s.ends)
	if n != 2 && n != 3 {
		t.Fatalf("series %v played %d games, want 2 or 3", pair, n)
	}
	if len(s.starts) != n || len(s.aborts) != 0 {
		t.Fatalf("series %v starts=%d aborts=%d ends=%d", pair, len(s.starts), len(s.aborts), n)
	}
	wins := [2]int{}
	for i, en := range s.ends {
		if en.Game != i+1 {
			t.Fatalf("series %v game %d ended out of order: %+v", pair, i+1, en)
		}
		if en.Winner != nil {
			if *en.Winner == pair[1] {
				wins[0]++
			} else {
				wins[1]++
			}
		}
	}
	if wins[0] < 2 && wins[1] < 2 && n != 3 {
		t.Fatalf("series %v undecided after %d games: %v", pair, n, wins)
	}
	table := s.starts[0].Table
	for i, en := range s.starts {
		if en.Game != i+1 || en.Attempt != 1 {
			t.Fatalf("series %v start %d = game %d attempt %d", pair, i, en.Game, en.Attempt)
		}
		if en.Table != table {
			t.Fatalf("series %v game %d moved table %s -> %s without a crash", pair, en.Game, table, en.Table)
		}
		var prevStart, prevEnd *event.Entry
		if i > 0 {
			prevStart, prevEnd = &s.starts[i-1], &s.ends[i-1]
		}
		checkChooserPlan(t, pair, en, prevStart, prevEnd)
	}
	for g := 2; g <= n; g++ {
		seen := map[int]bool{}
		for _, en := range s.sideb {
			if en.Game != g {
				continue
			}
			if en.Rev != 1 || seen[en.Seat] || (en.Seat != pair[1] && en.Seat != pair[2]) {
				t.Fatalf("series %v game %d sideboard %+v", pair, g, en)
			}
			seen[en.Seat] = true
		}
		if len(seen) != 2 {
			t.Fatalf("series %v game %d sideboarded by %d sides", pair, g, len(seen))
		}
	}
	if s.finished.Kind != event.KindSeriesEnded || s.finished.Score != wins {
		t.Fatalf("series %v SeriesEnded = %+v, wins %v", pair, s.finished, wins)
	}
	return wins
}

// TestBotOnlyBo3EventCompletes drives a bot-only best-of-three event to
// COMPLETE and checks every series' log: SeriesStarted 3, one series table
// per series (games 2/3 continue on it, chooser per MTR 2.2), bots
// sideboarding between games, SeriesEnded with the games' score; the
// series' table played exactly the series' games with the readied decks;
// the event state's series rows carry best_of 3 and the scores; and the
// standings equal event.ComputeStandings over the logged results.
func TestBotOnlyBo3EventCompletes(t *testing.T) {
	p := startGamePair(t, "", nil)
	req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	waitPhase(t, p.h, id, "COMPLETE", 50*time.Second)

	r := runOf(t, p.h, id)
	r.mu.Lock()
	log, st, standings := r.ev.Log(), r.stateFor(-1), r.ev.Standings()
	r.mu.Unlock()

	if st.BestOf != 3 {
		t.Fatalf("state best_of = %d, want 3", st.BestOf)
	}
	if len(log) == 0 || log[len(log)-1].Kind != event.KindEventEnded {
		t.Fatalf("event did not end: last entry %+v", log[len(log)-1])
	}
	perSeries := groupSeries(log)
	if len(perSeries) != 6 {
		t.Fatalf("%d series logged, want 6", len(perSeries))
	}
	seriesRows := map[[3]int]protocol.EventSeries{}
	for _, row := range st.Series {
		seriesRows[[3]int{row.Round, row.A, row.B}] = row
	}
	results := make([]event.SeriesResult, 0, len(perSeries))
	// Walk the series in (round, a, b) order -- the order the event itself
	// ended them in. Ranging the map directly fed ComputeStandings a random
	// order, and OMW%/OGW% are float means whose last bit depends on the
	// summation order, so the DeepEqual below failed at random (postmerge RED
	// 216758d3e: OGW ...967 vs ...969).
	pairs := make([][3]int, 0, len(perSeries))
	for pair := range perSeries {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i, j int) bool {
		for k := 0; k < 3; k++ {
			if pairs[i][k] != pairs[j][k] {
				return pairs[i][k] < pairs[j][k]
			}
		}
		return false
	})
	for _, pair := range pairs {
		s := perSeries[pair]
		wins := checkBo3Shape(t, pair, s)
		games := make([]event.GameOutcome, 0, len(s.ends))
		for _, en := range s.ends {
			switch {
			case en.Winner == nil:
				games = append(games, event.Draw)
			case *en.Winner == pair[1]:
				games = append(games, event.AWon)
			default:
				games = append(games, event.BWon)
			}
		}
		results = append(results, event.SeriesResult{
			Round: pair[0], A: pair[1], B: pair[2], Score: wins, Games: games,
		})
		row, ok := seriesRows[pair]
		if !ok || row.BestOf != 3 || row.State != "ended" || row.Score != wins {
			t.Fatalf("state series row %v = %+v, want ended best_of 3 score %v", pair, row, wins)
		}
		// The series table played exactly the series' games, match k with
		// the seats' readied decks (bots keep rev 1; slot 0 plays a's).
		mi, err := p.reg.Matches(host.TableID(s.starts[0].Table))
		if err != nil || len(mi) != len(s.ends) {
			t.Fatalf("Matches(%s) = %d matches (%v), want %d", s.starts[0].Table, len(mi), err, len(s.ends))
		}
		wantA, wantB := DeckID(id, pair[1], 1), DeckID(id, pair[2], 1)
		for _, m := range mi {
			if len(m.Seats) != 2 || m.Seats[0].DeckID != wantA || m.Seats[1].DeckID != wantB {
				t.Fatalf("series %v match decks = %s/%s, want %s/%s", pair,
					m.Seats[0].DeckID, m.Seats[1].DeckID, wantA, wantB)
			}
		}
	}
	if len(standings) != 4 {
		t.Fatalf("%d standings rows, want 4", len(standings))
	}
	if want := event.ComputeStandings(results); !reflect.DeepEqual(standings, want) {
		t.Fatalf("standings %+v, want ComputeStandings %+v", standings, want)
	}
}

// TestBotOnlyBo3EventIsDeterministic replays the same seed into two
// independent bot-only Bo3 events: their logs are byte-identical.
func TestBotOnlyBo3EventIsDeterministic(t *testing.T) {
	mk := func() []byte {
		p := startGamePair(t, "", nil)
		req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
		req.BestOf = 3
		resp := createOK(t, p.h, req)
		waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)
		r := runOf(t, p.h, resp.Event)
		r.mu.Lock()
		defer r.mu.Unlock()
		b, err := event.MarshalLog(r.ev.Log())
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first, second := mk(), mk()
	if !bytes.Equal(first, second) {
		t.Fatal("two bot-only Bo3 events with the same seed diverged")
	}
}

// TestBo1EventUnchanged pins the Bo1 shape against the Bo3 additions: the
// event state carries best_of 1 and every series row keeps exactly the E1
// fields (no game, games, chooser or waiting), and the log has no
// Sideboarded, no chooser and no best-of-three SeriesStarted.
func TestBo1EventUnchanged(t *testing.T) {
	p := startGamePair(t, "", nil)
	r := playBotEvent(t, p)

	r.mu.Lock()
	log, st := r.ev.Log(), r.stateFor(-1)
	r.mu.Unlock()

	if st.BestOf != 1 {
		t.Fatalf("state best_of = %d, want 1", st.BestOf)
	}
	for i, ser := range st.Series {
		if ser.BestOf != 1 || ser.Game != 0 || ser.Games != nil || ser.Chooser != nil || ser.Waiting != nil {
			t.Fatalf("series row %d grew Bo3 fields: %+v", i, ser)
		}
	}
	b, err := json.Marshal(&st)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"chooser"`, `"waiting"`, `"games"`} {
		if bytes.Contains(b, []byte(key)) {
			t.Fatalf("Bo1 event state JSON contains %s: %s", key, b)
		}
	}
	for _, en := range log {
		switch en.Kind {
		case event.KindSeriesStarted:
			if en.BestOf != 1 {
				t.Fatalf("SeriesStarted best_of %d, want 1", en.BestOf)
			}
		case event.KindGameStarted:
			if en.Chooser != nil {
				t.Fatalf("Bo1 GameStarted carries a chooser: %+v", en)
			}
		case event.KindSideboarded:
			t.Fatalf("Bo1 log has a Sideboarded: %+v", en)
		}
	}
}

// TestBo3CrashRetriesOnNewTable stalls every game (MaxDecisionsPerTurn 1)
// and caps attempts at 2, so every Bo3 game crashes twice and logs as a
// draw: each attempt restarts on a NEW table, the series plays its full
// three games and ends 0-0, and every game's chooser follows the rules —
// game 1 per attempt the attempt's own toss winner, later games the
// previous drawn game's chooser.
func TestBo3CrashRetriesOnNewTable(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, ho *host.Options) {
		eo.MaxAttempts = 2
		ho.MaxDecisionsPerTurn = 1
	})
	req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)

	r := runOf(t, p.h, resp.Event)
	r.mu.Lock()
	log := r.ev.Log()
	r.mu.Unlock()

	perSeries := groupSeries(log)
	if len(perSeries) != 6 {
		t.Fatalf("%d series logged, want 6", len(perSeries))
	}
	for pair, s := range perSeries {
		if s.started.Kind != event.KindSeriesStarted || s.started.BestOf != 3 {
			t.Fatalf("series %v SeriesStarted = %+v", pair, s.started)
		}
		if len(s.ends) != 3 || len(s.starts) != 6 || len(s.aborts) != 6 {
			t.Fatalf("series %v starts=%d aborts=%d ends=%d, want 6/6/3",
				pair, len(s.starts), len(s.aborts), len(s.ends))
		}
		// Every game is drawn, so games 2 and 3 re-use game 1's chooser
		// (MTR 2.2: a drawn game's chooser chooses again).
		var starts1 []event.Entry
		var end1 event.Entry
		for _, en := range s.starts {
			if en.Game == 1 {
				starts1 = append(starts1, en)
			}
		}
		for _, en := range s.ends {
			if en.Game == 1 {
				end1 = en
			}
		}
		if len(starts1) != 2 || end1.Kind != event.KindGameEnded {
			t.Fatalf("series %v game 1 starts=%d end=%+v", pair, len(starts1), end1)
		}
		tables := map[string]bool{}
		// Log order per game g: start(a1), abort(a1), start(a2), abort(a2), end.
		for g := 1; g <= 3; g++ {
			var gStarts, gAborts []event.Entry
			for _, en := range s.starts {
				if en.Game == g {
					gStarts = append(gStarts, en)
				}
			}
			for _, en := range s.aborts {
				if en.Game == g {
					gAborts = append(gAborts, en)
				}
			}
			if len(gStarts) != 2 || len(gAborts) != 2 {
				t.Fatalf("series %v game %d starts=%d aborts=%d", pair, g, len(gStarts), len(gAborts))
			}
			for i, en := range gStarts {
				if en.Attempt != i+1 {
					t.Fatalf("series %v game %d start %d has attempt %d", pair, g, i, en.Attempt)
				}
				if tables[en.Table] {
					t.Fatalf("series %v game %d attempt %d reused table %s", pair, g, en.Attempt, en.Table)
				}
				tables[en.Table] = true
				var prevStart, prevEnd *event.Entry
				if g > 1 {
					prevStart, prevEnd = &starts1[len(starts1)-1], &end1
				}
				checkChooserPlan(t, pair, en, prevStart, prevEnd)
			}
			for _, en := range gAborts {
				if en.Reason != "crashed" || en.Attempt > 2 {
					t.Fatalf("series %v game %d abort %+v", pair, g, en)
				}
			}
		}
		for _, en := range s.ends {
			if en.Winner != nil || en.Head == "" {
				t.Fatalf("series %v GameEnded = %+v, want a headless draw with a head", pair, en)
			}
		}
		if s.finished.Kind != event.KindSeriesEnded || s.finished.Score != [2]int{0, 0} {
			t.Fatalf("series %v SeriesEnded = %+v", pair, s.finished)
		}
	}
}

// seriesRes is one Host.Series call's result, for parked-call tests.
type seriesRes struct {
	g  host.SeriesGame
	ok bool
}

// parkSeries calls h.Series on a goroutine and returns the result channel.
func parkSeries(h *Host, ctx context.Context, id host.TableID, k int) <-chan seriesRes {
	ch := make(chan seriesRes, 1)
	go func() {
		g, ok, _ := h.Series(ctx, id, k)
		ch <- seriesRes{g: g, ok: ok}
	}()
	return ch
}

// assertParked fails if the parked call answers within d.
// parkDwell is how long a "still parked" check waits for an early answer. It
// is a dwell window, not a threshold: a correct park never answers, so load
// can only make the check weaker, never turn PASS into FAIL (the wall-clock
// ratchet, testkit/testutil TestNoNewSubSecondWallClockThresholdInTests).
const parkDwell = time.Second / 2

func assertParked(t *testing.T, ch <-chan seriesRes, d time.Duration) {
	t.Helper()
	select {
	case res := <-ch:
		t.Fatalf("parked series call answered early: %+v (ok %v)", res.g, res.ok)
	case <-time.After(d):
	}
}

// assertAnswered fails unless the parked call answers within d.
func assertAnswered(t *testing.T, ch <-chan seriesRes, d time.Duration) seriesRes {
	t.Helper()
	select {
	case res := <-ch:
		return res
	case <-time.After(d):
		t.Fatalf("parked series call never answered within %s", d)
		return seriesRes{}
	}
}

// TestSeriesHookWaitsAndCloseReleases drives the Host.Series hook against
// a live Bo3 series table whose human seat never acts, so game 1 stays in
// flight: match 1 plans ready with the two saved decks and no chooser (the
// seeded toss decides), match 2 parks on the run until the log moves or
// the call is released, an unknown or non-series table answers no, a
// cancelled context returns promptly, and Host.Close releases the parked
// call and returns.
func TestSeriesHookWaitsAndCloseReleases(t *testing.T) {
	dir := t.TempDir()
	mut := func(eo *Options, _ *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	}
	p := startGamePair(t, dir, mut)
	req := oneHumanReq()
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	r := runOf(t, p.h, id)
	r.mu.Lock()
	tableID, seriesOK := r.seriesTable[[2]int{0, 3}]
	r.mu.Unlock()
	if !seriesOK {
		t.Fatal("seat 0's series has no table")
	}

	// Match 1's plan is ready: both saved decks, chooser -1 (the seeded
	// toss decides).
	res := assertAnswered(t, parkSeries(p.h, context.Background(), tableID, 1), 10*time.Second)
	if !res.ok || res.g.Chooser != -1 || len(res.g.Decks) != 2 ||
		res.g.Decks[0] != DeckID(id, 0, 1) || res.g.Decks[1] != DeckID(id, 3, 1) {
		t.Fatalf("match 1 plan = %+v ok=%v", res.g, res.ok)
	}

	// An unknown table and a non-series table answer no.
	if g, ok, err := p.h.Series(context.Background(), "no-such-table", 1); ok || err != nil || g.Chooser != 0 {
		t.Fatalf("unknown table plan = %+v ok=%v err=%v", g, ok, err)
	}
	botResp := createOK(t, p.h, botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot"))
	waitPhase(t, p.h, botResp.Event, "ROUND1", 50*time.Second)
	rb1 := runOf(t, p.h, botResp.Event)
	rb1.mu.Lock()
	bo1Tables := map[host.TableID]bool{}
	for _, att := range rb1.ev.InFlight() {
		bo1Tables[host.TableID(att.Table)] = true
	}
	rb1.mu.Unlock()
	for tid := range bo1Tables {
		if g, ok, err := p.h.Series(context.Background(), tid, 1); ok || err != nil {
			t.Fatalf("Bo1 table %s plan = %+v ok=%v err=%v", tid, g, ok, err)
		}
	}

	// Match 2 parks while game 1 is in flight, and a cancelled context
	// unblocks it.
	ctx, cancel := context.WithCancel(context.Background())
	ch := parkSeries(p.h, ctx, tableID, 2)
	assertParked(t, ch, parkDwell)
	cancel()
	if res := assertAnswered(t, ch, 10*time.Second); res.ok {
		t.Fatalf("cancelled park answered ok: %+v", res.g)
	}

	// A second park is released by Close: the call answers no and Close
	// returns.
	ch = parkSeries(p.h, context.Background(), tableID, 2)
	assertParked(t, ch, parkDwell)
	closed := make(chan error, 1)
	go func() { closed <- p.h.Close() }()
	if res := assertAnswered(t, ch, 10*time.Second); res.ok {
		t.Fatalf("Close-released park answered ok: %+v", res.g)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Close did not return with a parked series call")
	}
}

// cutResumeDir builds a fresh disk store that holds the source event's
// meta and deck files plus a LOG PREFIX of a played-out event: the first n
// entries of log. A new pair opened on the result resumes the event from
// the prefix, exactly as a restart that crashed after that entry would.
func cutResumeDir(t *testing.T, srcDir, id string, log []event.Entry, n int) string {
	t.Helper()
	dst := t.TempDir()
	ev := filepath.Join(dst, id)
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile(filepath.Join(srcDir, id, "event.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ev, "event.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(ev, "decks"), os.DirFS(filepath.Join(srcDir, id, "decks"))); err != nil {
		t.Fatal(err)
	}
	lb, err := event.MarshalLog(log[:n])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ev, "log.jsonl"), lb, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// playBotBo3 runs a bot-only Bo3 event to COMPLETE on a disk store and
// returns the pair, the event id and the full log.
func playBotBo3(t *testing.T, dir string) (*gamePair, string, []event.Entry) {
	t.Helper()
	p := startGamePair(t, dir, nil)
	req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	waitPhase(t, p.h, id, "COMPLETE", 50*time.Second)
	r := runOf(t, p.h, id)
	r.mu.Lock()
	log := r.ev.Log()
	r.mu.Unlock()
	return p, id, log
}

// assertPrefixReplayed asserts the resumed log carries the prefix entries
// verbatim before anything new.
func assertPrefixReplayed(t *testing.T, prefix, log2 []event.Entry, n int) {
	t.Helper()
	if len(log2) <= n {
		t.Fatalf("resumed log %d entries, want more than the %d-entry prefix", len(log2), n)
	}
	for i := 0; i < n; i++ {
		if !reflect.DeepEqual(prefix[i], log2[i]) {
			t.Fatalf("resumed log diverges at %d: %+v vs %+v", i, prefix[i], log2[i])
		}
	}
}

// TestResumeBetweenGamesContinuesOnANewTable restarts a bot-only Bo3 event
// from a store whose log ends at round-1 series (0,3)'s first Sideboarded
// pair: the series resumes on a NEW table (the first host's series table
// died with it), plays its game 2 there, and the event reaches COMPLETE
// with every series' log shape intact.
func TestResumeBetweenGamesContinuesOnANewTable(t *testing.T) {
	dir := t.TempDir()
	p, id, log := playBotBo3(t, dir)

	// Cut after (0,3)'s first Sideboarded pair (game 2's two sides).
	cut := -1
	seen := 0
	for i, en := range log {
		if en.Kind == event.KindSideboarded && en.Round == 1 && en.A == 0 && en.B == 3 {
			seen++
			cut = i
			if seen == 2 {
				break
			}
		}
	}
	if seen != 2 {
		t.Fatalf("round-1 (0,3) Sideboarded pair not on the log (saw %d)", seen)
	}
	for _, en := range log[cut-1 : cut+1] {
		if en.Game != 2 || en.Rev != 1 || (en.Seat != 0 && en.Seat != 3) {
			t.Fatalf("cut entries are not (0,3) game 2 rev-1 sideboards: %+v", en)
		}
	}
	prefixTables := map[string]bool{}
	for _, en := range log[:cut+1] {
		if en.Kind == event.KindGameStarted {
			prefixTables[en.Table] = true
		}
	}
	dst := cutResumeDir(t, dir, id, log, cut+1)
	p.h.Close()
	p.reg.Close()

	p2 := startGamePair(t, dst, nil)
	waitPhase(t, p2.h, id, "COMPLETE", 50*time.Second)
	r2 := runOf(t, p2.h, id)
	r2.mu.Lock()
	log2 := r2.ev.Log()
	r2.mu.Unlock()
	assertPrefixReplayed(t, log, log2, cut+1)

	// The first new GameStarted is (0,3) game 2's attempt 1, on a fresh
	// table that hosts game 2 (not the dead game-1 table).
	var first event.Entry
	for _, en := range log2[cut+1:] {
		if en.Kind == event.KindGameStarted {
			first = en
			break
		}
	}
	if first.Kind != event.KindGameStarted || first.Round != 1 || first.A != 0 || first.B != 3 ||
		first.Game != 2 || first.Attempt != 1 {
		t.Fatalf("first resumed GameStarted = %+v, want (0,3) game 2 attempt 1", first)
	}
	if want := fmt.Sprintf("%s-r1-s03-g2-a1", id); first.Table != want {
		t.Fatalf("first resumed GameStarted table %s, want %s", first.Table, want)
	}
	if prefixTables[first.Table] {
		t.Fatalf("resumed run reused dead table %s", first.Table)
	}
	for _, en := range log2[cut+1:] {
		if en.Kind == event.KindGameStarted && prefixTables[en.Table] {
			t.Fatalf("resumed run reused dead table %s: %+v", en.Table, en)
		}
	}

	// Every series still plays out in shape: 2 or 3 ended games, decided
	// or full, SeriesEnded with the games' score, and each start's chooser
	// per MTR 2.2 (across the table change for (0,3)).
	perSeries := groupSeries(log2)
	if len(perSeries) != 6 {
		t.Fatalf("%d series logged, want 6", len(perSeries))
	}
	for pair, s := range perSeries {
		if s.started.Kind != event.KindSeriesStarted || s.started.BestOf != 3 {
			t.Fatalf("series %v SeriesStarted = %+v", pair, s.started)
		}
		if len(s.aborts) != 0 || len(s.ends) < 2 || len(s.ends) > 3 {
			t.Fatalf("series %v aborts=%d ends=%d", pair, len(s.aborts), len(s.ends))
		}
		wins := [2]int{}
		for _, en := range s.ends {
			if en.Winner != nil {
				if *en.Winner == pair[1] {
					wins[0]++
				} else {
					wins[1]++
				}
			}
		}
		if wins[0] < 2 && wins[1] < 2 && len(s.ends) != 3 {
			t.Fatalf("series %v undecided after %d games: %v", pair, len(s.ends), wins)
		}
		if s.finished.Kind != event.KindSeriesEnded || s.finished.Score != wins {
			t.Fatalf("series %v SeriesEnded = %+v, wins %v", pair, s.finished, wins)
		}
		startsByGame := map[int][]event.Entry{}
		endByGame := map[int]*event.Entry{}
		for _, en := range s.ends {
			e := en
			endByGame[en.Game] = &e
		}
		for _, en := range s.starts {
			startsByGame[en.Game] = append(startsByGame[en.Game], en)
		}
		for g := 1; g <= len(s.ends); g++ {
			sts := startsByGame[g]
			if len(sts) != 1 {
				t.Fatalf("series %v game %d has %d starts", pair, g, len(sts))
			}
			en := sts[0]
			if en.Attempt != 1 {
				t.Fatalf("series %v game %d start attempt %d", pair, g, en.Attempt)
			}
			var prevStart, prevEnd *event.Entry
			if g > 1 {
				prevStart, prevEnd = &startsByGame[g-1][0], endByGame[g-1]
			}
			checkChooserPlan(t, pair, en, prevStart, prevEnd)
		}
	}
}

// TestResumeMidGameAbortsAndRetries restarts a bot-only Bo3 event from a
// store whose log ends at round-1 series (0,3) game 2 attempt 1's
// GameStarted: the resumed run aborts the caught-in-flight attempt
// ("server restart") and retries game 2 as attempt 2 on a fresh table with
// a different game seed.
func TestResumeMidGameAbortsAndRetries(t *testing.T) {
	dir := t.TempDir()
	p, id, log := playBotBo3(t, dir)

	cut := -1
	for i, en := range log {
		if en.Kind == event.KindGameStarted && en.Round == 1 && en.A == 0 && en.B == 3 &&
			en.Game == 2 && en.Attempt == 1 {
			cut = i
			break
		}
	}
	if cut < 0 {
		t.Fatal("round-1 (0,3) game 2 attempt 1 never started")
	}
	seed1 := log[cut].GameSeed
	dst := cutResumeDir(t, dir, id, log, cut+1)
	p.h.Close()
	p.reg.Close()

	p2 := startGamePair(t, dst, nil)
	waitPhase(t, p2.h, id, "COMPLETE", 50*time.Second)
	r2 := runOf(t, p2.h, id)
	r2.mu.Lock()
	log2 := r2.ev.Log()
	r2.mu.Unlock()
	assertPrefixReplayed(t, log, log2, cut+1)

	// The resumed tail: the caught-in-flight attempt aborted with the
	// restart reason, then game 2 retried as attempt 2 on a new table
	// (…-g2-a2) with a different seed.
	aborted, retried := false, false
	for _, en := range log2[cut+1:] {
		if !aborted {
			if en.Kind != event.KindGameAborted {
				t.Fatalf("first resumed entry = %+v, want the in-flight game aborted", en)
			}
			if en.Round != 1 || en.A != 0 || en.B != 3 || en.Game != 2 ||
				en.Attempt != 1 || en.Reason != "server restart" {
				t.Fatalf("restart abort = %+v", en)
			}
			aborted = true
			continue
		}
		if en.Kind != event.KindGameStarted {
			continue
		}
		if en.Round != 1 || en.A != 0 || en.B != 3 || en.Game != 2 || en.Attempt != 2 {
			t.Fatalf("retried GameStarted = %+v, want (0,3) game 2 attempt 2", en)
		}
		if want := fmt.Sprintf("%s-r1-s03-g2-a2", id); en.Table != want {
			t.Fatalf("retried game 2 table %s, want %s", en.Table, want)
		}
		if en.GameSeed == seed1 {
			t.Fatal("retried game 2 reused attempt 1's game seed")
		}
		retried = true
		break
	}
	if !aborted || !retried {
		t.Fatal("resumed log never aborted and retried the caught-in-flight game")
	}
}

// TestBo3CreateValidation pins the create-time best_of grammar: 2 and 4
// are bad requests, 0, 1 and 3 are accepted and reported back in the
// event's state (0 as 1).
func TestBo3CreateValidation(t *testing.T) {
	h, _ := newHost(t, nil)
	req := botEventReq("bot", "bot", "bot", "bot")
	for _, bad := range []int{2, 4, -1} {
		req.BestOf = bad
		_, err := h.CreateEvent(req)
		e, ok := err.(*Error)
		if !ok || e.Status() != 400 {
			t.Fatalf("CreateEvent best_of %d: %v, want a 400", bad, err)
		}
	}
	for _, good := range []int{0, 1, 3} {
		req.BestOf = good
		resp, err := h.CreateEvent(req)
		if err != nil {
			t.Fatalf("CreateEvent best_of %d: %v", good, err)
		}
		st := stateOf(t, h, resp.Event, "")
		want := good
		if want == 0 {
			want = 1
		}
		if st.BestOf != want {
			t.Fatalf("best_of %d: state reports %d, want %d", good, st.BestOf, want)
		}
	}
}
