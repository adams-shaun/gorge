package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// The feedback snapshot (task fbrepro1) is exercised end to end here, on a
// real registry playing a real match, because the thing under test is the
// whole chain: a POST naming a table must leave match.json/log.json/
// view.json in the report directory, and the log must replay.

// gateSeat is seat 0's bot behind a test gate: every decision is signalled
// to the test, which releases it one at a time — so the test holds the
// match at a known point (parked on a seat-0 decision) and advances it
// decision by decision.
type gateSeat struct {
	bot     seat.Seat
	reached chan struct{}
	release chan struct{}
}

func (g *gateSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	// Both the signal and the wait must yield to ctx: once the test stops
	// reading reached (or Close cancels the match's context) the gate has to
	// unblock, or Close's wg.Wait would hang on a parked seat forever.
	select {
	case g.reached <- struct{}{}:
	case <-ctx.Done():
		return decision.Intent{}, ctx.Err()
	}
	select {
	case <-g.release:
	case <-ctx.Done():
		return decision.Intent{}, ctx.Err()
	}
	return g.bot.Decide(ctx, v, d)
}

// gatedRegistry builds a four-seat registry whose seat 0 is gated. It
// returns the gate, an advance helper and the registry; the table is
// already started. Cleanup closes the registry, which cancels the match's
// context and releases any parked gate decision.
func gatedRegistry(t *testing.T, n int) (*gateSeat, func(int), *host.Registry) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, n)
	by := map[string][]*cards.Card{}
	for i, d := range decks {
		by[names[i]] = d
	}
	load := func(name string) (host.Deck, error) {
		cs, ok := by[name]
		if !ok {
			return host.Deck{}, host.ErrNotFound
		}
		return host.Deck{Name: name, Cards: cs}, nil
	}
	gate := &gateSeat{bot: seat.NewBot(1), reached: make(chan struct{}), release: make(chan struct{})}
	r, err := host.New(host.Options{
		LoadDeck: load,
		Seats: func(seatNames []string, seed uint64) []seat.Seat {
			out := make([]seat.Seat, len(seatNames))
			out[0] = gate
			for i := 1; i < len(seatNames); i++ {
				out[i] = seat.NewBot(seed ^ uint64(i+1))
			}
			return out
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "Table t1", Seats: n,
		Decks: []string{"a", "b", "c", "d"}[:n], Seed: 42, Spectator: view.Public}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	advance := func(times int) {
		for i := 0; i < times; i++ {
			select {
			case <-gate.reached:
			case <-time.After(30 * time.Second):
				t.Fatalf("seat 0 never reached decision %d", i+1)
			}
			gate.release <- struct{}{}
		}
	}
	return gate, advance, r
}

// feedbackSnapshotFiles reads the three snapshot files a report directory
// should hold, failing when one is missing.
func feedbackSnapshotFiles(t *testing.T, dir string) (matchRaw, logRaw, viewRaw []byte) {
	t.Helper()
	for _, f := range []struct {
		name string
		dst  *[]byte
	}{{"match.json", &matchRaw}, {"log.json", &logRaw}, {"view.json", &viewRaw}} {
		b, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		*f.dst = b
	}
	return
}

// feedbackLog unmarshals log.json the way a repro tool would.
type feedbackLog struct {
	events.Log
	Head        string `json:"head"`
	IntentCount int    `json:"intent_count"`
	Turn        int32  `json:"turn"`
	Step        string `json:"step"`
	Priority    int    `json:"priority"`
	Active      int    `json:"active"`
}

// feedbackMatchJSON unmarshals match.json the way a repro tool would: the
// sidecar's config plus the deck contents.
type feedbackMatchJSON struct {
	Table       string     `json:"table"`
	Match       int        `json:"match"`
	Seed        uint64     `json:"seed"`
	Names       []string   `json:"names"`
	PlayerNames []string   `json:"player_names"`
	Decks       []string   `json:"decks"`
	DeckCards   [][]string `json:"deck_cards"`
	Mulligans   int        `json:"mulligans"`
	Format      string     `json:"format"`
}

// rebuildConfig is the reproduction path a report consumer takes: a
// rules.Config rebuilt from match.json alone (plus the card index that maps
// the recorded card names back to cards), ready for replay.Replay.
func rebuildConfig(t *testing.T, m feedbackMatchJSON, byName map[string]*cards.Card) rules.Config {
	t.Helper()
	decks := make([][]*cards.Card, len(m.DeckCards))
	for i, names := range m.DeckCards {
		decks[i] = make([]*cards.Card, len(names))
		for j, n := range names {
			c, ok := byName[n]
			if !ok {
				t.Fatalf("match.json names card %q, which the card index does not know", n)
			}
			decks[i][j] = c
		}
	}
	cfg := rules.Config{Seed: m.Seed, Names: m.Names, PlayerNames: m.PlayerNames, Decks: decks, Mulligans: m.Mulligans}
	if m.Format == "commander" {
		cfg.Format = rules.FormatCommander
	}
	return cfg
}

// TestFeedbackCapturesAReplayableSnapshot: a bot match, advanced a few
// decisions, then a report naming the table — match.json, log.json and
// view.json land beside it, the recorded head is the chain hash at exactly
// the recorded prefix, and replaying that log against a Config rebuilt from
// match.json reproduces the same head.
func TestFeedbackCapturesAReplayableSnapshot(t *testing.T) {
	_, advance, r := gatedRegistry(t, 4)
	advance(12) // "advance it": a real mid-match prefix, not genesis

	_, decks := testutil.SampleDecks(t, 4)
	byName := map[string]*cards.Card{}
	for _, d := range decks {
		for _, c := range d {
			byName[c.Faces[0].Name] = c
		}
	}

	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	// The table id comes from the url's /t/ route; the seat from the url's
	// seat= query parameter, the shape today's client reports.
	rec := postFeedback(t, fs, "Gisa made no zombies", "http://localhost:8080/t/t1?seat=0", nil)
	if rec.Code != 201 {
		t.Fatalf("status = %d (body %q)", rec.Code, rec.Body.String())
	}
	dirs := storedReports(t, fs)
	if len(dirs) != 1 {
		t.Fatalf("stored %d reports", len(dirs))
	}
	if got := readReport(t, dirs[0]).Snapshot; !strings.HasPrefix(got, "captured: match.json log.json view.json") {
		t.Fatalf("snapshot status %q", got)
	}
	matchRaw, logRaw, viewRaw := feedbackSnapshotFiles(t, dirs[0])
	if len(viewRaw) == 0 {
		t.Fatal("view.json is empty")
	}

	var lg feedbackLog
	if err := json.Unmarshal(logRaw, &lg); err != nil {
		t.Fatal(err)
	}
	var m feedbackMatchJSON
	if err := json.Unmarshal(matchRaw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Table != "t1" || len(m.DeckCards) != 4 || len(m.DeckCards[0]) == 0 {
		t.Fatalf("match.json: %+v", m)
	}
	// The deck contents are in deck order and are exactly what the loader
	// served: seat i of match 1 plays Decks[(i+1)%4] = b, c, d, a.
	want := [][]*cards.Card{decks[1], decks[2], decks[3], decks[0]}
	for i, names := range m.DeckCards {
		if len(names) != len(want[i]) {
			t.Fatalf("seat %d recorded %d cards, deck has %d", i, len(names), len(want[i]))
		}
		for j, n := range names {
			if n != want[i][j].Faces[0].Name {
				t.Fatalf("seat %d card %d = %q, want %q", i, j, n, want[i][j].Faces[0].Name)
			}
		}
	}
	if lg.IntentCount != len(lg.Intents) {
		t.Fatalf("intent_count %d, log carries %d intents", lg.IntentCount, len(lg.Intents))
	}
	if lg.IntentCount == 0 {
		t.Fatal("snapshot taken with no intents recorded")
	}
	// The recorded head is the chain hash at exactly the recorded prefix.
	chain := &events.Log{Seed: lg.Seed, Events: lg.Events}
	if got := chain.HeadAt(len(lg.Events)); got != lg.Head {
		t.Fatalf("head %q, but the recorded prefix hashes to %q", lg.Head, got)
	}
	// The gate: the log replays against the config match.json rebuilt, to
	// the same head.
	cfg := rebuildConfig(t, m, byName)
	e, err := replay.Replay(&lg.Log, cfg)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := e.L.Head(); got != lg.Head {
		t.Fatalf("replayed head %q, recorded %q", got, lg.Head)
	}
}

// TestFeedbackViewForSeat0CarriesNoOtherSeatsHiddenInfo submits from seat 0
// at the match's very first decision — before any other seat has acted, so
// nothing of theirs can be on a public zone — and checks the redaction the
// brief demands: another seat's card identities never appear, while seat
// 0's own hand does.
func TestFeedbackViewForSeat0CarriesNoOtherSeatsHiddenInfo(t *testing.T) {
	_, advance, r := gatedRegistry(t, 4)
	advance(1)

	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	// The table id comes from the url; the seat from the explicit form
	// field, the shape the client ticket will send.
	rec := postFeedbackSeat(t, fs, "creatures are 2/2", "http://localhost:8080/t/t1", "0")
	if rec.Code != 201 {
		t.Fatalf("status = %d (body %q)", rec.Code, rec.Body.String())
	}
	dirs := storedReports(t, fs)
	if len(dirs) != 1 {
		t.Fatalf("stored %d reports", len(dirs))
	}
	if got := readReport(t, dirs[0]).Snapshot; !strings.HasPrefix(got, "captured") {
		t.Fatalf("snapshot status %q", got)
	}
	_, _, viewRaw := feedbackSnapshotFiles(t, dirs[0])
	var v view.View
	if err := json.Unmarshal(viewRaw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Viewer != 0 || v.Visibility != "seat" {
		t.Fatalf("viewer %d visibility %q", v.Viewer, v.Visibility)
	}
	for i, p := range v.Players {
		if i == 0 {
			if p.Hand == nil || len(p.Hand) == 0 {
				t.Fatalf("seat 0's own hand is empty (%v) — the view is not seat-scoped", p.Hand)
			}
			continue
		}
		if p.Hand != nil {
			// A redacted hand marshals as null; [] would mean the other
			// seat's hand shape was exposed as a public empty.
			t.Errorf("seat %d's hand = %v, want nil (hidden)", i, p.Hand)
		}
	}
	// The direct reading of the brief: no card identity from another seat's
	// hand or library anywhere in the file. Seat i of match 1 plays deck
	// (i+1)%4 (host/table.go's rotation), so the reporter's own deck is the
	// Forest deck and the deck at seat 1 is the Plains deck — and at this
	// point, seat 0's first decision before any other seat has acted,
	// nothing of any other seat's can be on a public zone. So "Plains" (the
	// neighbour's basic land, present in every one of their cards' names)
	// must be absent from the whole file, while the reporter's own "Forest"
	// must be present.
	if strings.Contains(string(viewRaw), "Plains") {
		t.Error("seat 0's view.json contains a Plains (another seat's) card identity")
	}
	// Positive control: seat 0's own cards do appear.
	if !strings.Contains(string(viewRaw), "Forest") {
		t.Error("seat 0's view.json does not contain seat 0's own Forest")
	}
}

// TestFeedbackUnknownTableStillStoresTheReport pins the report-never-lost
// rule: a report naming a table that does not exist (or no table at all) is
// stored whole, returns 201, and says in report.json why there is no
// snapshot.
func TestFeedbackUnknownTableStillStoresTheReport(t *testing.T) {
	_, _, r := gatedRegistry(t, 4) // a live registry, but no table "nope"
	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	// An unknown table.
	rec := postFeedback(t, fs, "crash on draw", "http://localhost:8080/t/nope?seat=1", nil)
	if rec.Code != 201 {
		t.Fatalf("unknown table: status = %d (body %q)", rec.Code, rec.Body.String())
	}
	dirs := storedReports(t, fs)
	if len(dirs) != 1 {
		t.Fatalf("stored %d reports", len(dirs))
	}
	if got := readReport(t, dirs[0]).Snapshot; !strings.HasPrefix(got, "unavailable:") {
		t.Fatalf("snapshot status %q, want unavailable: ...", got)
	}
	entries, err := os.ReadDir(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") && e.Name() != "report.json" {
			t.Errorf("unknown-table report wrote %s", e.Name())
		}
	}

	// A report naming no table at all snapshots nothing and says nothing.
	rec = postFeedback(t, fs, "the lobby looks odd", "http://localhost:8080/", nil)
	if rec.Code != 201 {
		t.Fatalf("no table: status = %d (body %q)", rec.Code, rec.Body.String())
	}
	for _, d := range storedReports(t, fs) {
		rep := readReport(t, d)
		if rep.Text != "the lobby looks odd" {
			continue
		}
		if got := rep.Snapshot; got != "" {
			t.Fatalf("no-table report snapshot status %q, want empty", got)
		}
		return
	}
	t.Fatal("the no-table report was not stored")

	// An unparseable seat is a reason, not a 400. Report directories are
	// timestamped-and-randomised, so find the one by its text, not its
	// position.
	rec = postFeedbackSeat(t, fs, "seat went wrong", "http://localhost:8080/t/t1", "banana")
	if rec.Code != 201 {
		t.Fatalf("bad seat: status = %d (body %q)", rec.Code, rec.Body.String())
	}
	for _, d := range storedReports(t, fs) {
		rep := readReport(t, d)
		if rep.Text != "seat went wrong" {
			continue
		}
		if !strings.HasPrefix(rep.Snapshot, "unavailable:") {
			t.Fatalf("bad-seat snapshot status %q, want unavailable: ...", rep.Snapshot)
		}
		return
	}
	t.Fatal("the bad-seat report was not stored")
}

// TestFeedbackSnapshotDuringAnInFlightMatch hammers the snapshot while the
// match is actively resolving bursts, so the read path (Registry.
// SnapshotForFeedback) runs concurrently with the match goroutine's writes.
// It is written for the merge gate's `go test -race ./cmd/gorged/` run
// (Ruling FL-67: the -race gate runs by hand, one package per invocation),
// where any lock-discipline slip between the two goroutines fails loudly;
// without -race it still proves the snapshot is consistent (the copied log
// only ever grows, and every captured prefix replays).
func TestFeedbackSnapshotDuringAnInFlightMatch(t *testing.T) {
	gate, advance, r := gatedRegistry(t, 4)
	// Wait for the first decision so the match exists and is parked mid-
	// game before the reader starts: a snapshot of a table between matches
	// is a different (error) path, tested elsewhere.
	select {
	case <-gate.reached:
	case <-time.After(30 * time.Second):
		t.Fatal("seat 0 never reached its first decision")
	}
	gate.release <- struct{}{}
	stop := make(chan struct{})
	errc := make(chan error, 1)
	type captured struct{ nEvents, nIntents int }
	captures := make(chan captured, 1024)
	go func() {
		defer close(errc)
		defer close(captures)
		seat0 := state.PlayerID(0)
		first := -1
		for {
			select {
			case <-stop:
				errc <- nil
				return
			default:
			}
			snap, err := r.SnapshotForFeedback("t1", &seat0)
			if err != nil {
				errc <- err
				return
			}
			n := len(snap.Log.Events)
			if first < 0 {
				first = n
			}
			if n < first {
				errc <- fmt.Errorf("snapshot log shrank: %d after %d", n, first)
				return
			}
			select {
			case captures <- captured{n, len(snap.Log.Intents)}:
			default:
			}
		}
	}()
	advance(25)
	close(stop)
	if err := <-errc; err != nil {
		t.Fatalf("concurrent snapshot failed: %v", err)
	}
	var last captured
	n := 0
	for c := range captures {
		last = c
		n++
	}
	if n == 0 {
		t.Fatal("no concurrent capture completed")
	}
	// The match actually moved while being read: the last capture saw more
	// of the log than the first decision's burst.
	if last.nEvents <= 40 {
		t.Fatalf("last capture saw only %d events — the match never advanced under the reader", last.nEvents)
	}
	t.Logf("%d concurrent captures, last at %d events / %d intents", n, last.nEvents, last.nIntents)
}

// postFeedbackSeat drives the handler with an explicit seat form field, the
// shape the client ticket (fbrepro3) will send.
func postFeedbackSeat(t *testing.T, fs *feedbackStore, text, url, seat string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("text", text); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("url", url); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("seat", seat); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	fs.submit(rec, req)
	return rec
}
