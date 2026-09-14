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
// sidecar's config plus the deck contents and the token scripts.
type feedbackMatchJSON struct {
	Table        string            `json:"table"`
	Match        int               `json:"match"`
	Seed         uint64            `json:"seed"`
	Names        []string          `json:"names"`
	PlayerNames  []string          `json:"player_names"`
	Decks        []string          `json:"decks"`
	DeckCards    [][]string        `json:"deck_cards"`
	Tokens       map[string]string `json:"tokens"`
	TokensUnread []string          `json:"tokens_unread"`
	Mulligans    int               `json:"mulligans"`
	Format       string            `json:"format"`
}

// rebuildConfig is the reproduction path a report consumer takes: a
// rules.Config rebuilt from match.json alone (plus the card index that maps
// the recorded card names back to cards, and the recorded token scripts
// recompiled through the corpus's own parse/link/intrinsics pipeline), ready
// for replay.Replay.
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
	for stem, src := range m.Tokens {
		c := compileToken(t, stem, src)
		if cfg.Tokens == nil {
			cfg.Tokens = map[string]*cards.Card{}
		}
		cfg.Tokens[stem] = c
	}
	return cfg
}

// compileToken recompiles one recorded token script the way the corpus's
// own compileScripts does: ParseBytes, then Link (which also expands
// keywords), then ApplyIntrinsics. ParseBytes derives every face itself, so
// the rebuilt card is the one the match played with, not a zero-derived
// shell.
func compileToken(t *testing.T, stem, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("tokenscripts/"+stem+".txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("token script %q: %v", stem, diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("token script %q: link: %v", stem, d)
	}
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
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

	// A report with neither a URL nor an explicit table still records why
	// there is no snapshot; an omitted status would recreate the ambiguity
	// this field exists to remove.
	rec = postFeedback(t, fs, "the lobby looks odd", "", nil)
	if rec.Code != 201 {
		t.Fatalf("no table: status = %d (body %q)", rec.Code, rec.Body.String())
	}
	foundNoTable := false
	for _, d := range storedReports(t, fs) {
		rep := readReport(t, d)
		if rep.Text != "the lobby looks odd" {
			continue
		}
		foundNoTable = true
		if got := rep.Snapshot; got != "unavailable: no table named" {
			t.Fatalf("no-table report snapshot status %q, want %q", got, "unavailable: no table named")
		}
	}
	if !foundNoTable {
		t.Fatal("the no-table report was not stored")
	}

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

// tokenFixture writes an original-text token script to disk — the shape
// production's corpus compiles from, a real file the compiled Card's Path
// names, which is what the snapshot's token capture reads back — and returns
// it compiled through the corpus's own pipeline. The text is authored here,
// never copied from the corpus's GPL-3.0 tokenscripts (echoing the real
// r_1_1_goblin stem name, the way effects/token_test.go's fixtures do).
func tokenFixture(t *testing.T) *cards.Card {
	t.Helper()
	return tokenCardOnDisk(t, "r_1_1_goblin", "Name:Goblin Token\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
}

// tokenCardOnDisk writes one token script under a real temp-file path and
// compiles it with ParseBytes + Link + ApplyIntrinsics — the exact pipeline
// cards.compileScripts runs, so the rebuilt card is what a replay needs.
func tokenCardOnDisk(t *testing.T, stem, src string) *cards.Card {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tokenscripts")
	p := filepath.Join(dir, stem+".txt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, diags := cards.ParseBytes(p, []byte(src))
	if len(diags) != 0 {
		t.Fatalf("token script %q: %v", stem, diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("token script %q: link: %v", stem, d)
	}
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// inlineCard compiles one authored card script, the way testutil's own deck
// builder does.
func inlineCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("test.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("card script %q: %v", src, diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("card script %q: link: %v", src, d)
	}
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// TestFeedbackReplaysAMatchThatCreatesTokens pins the token half of the
// snapshot: a match whose seats actually cast a token spell produces a
// TokenCreate event, match.json carries the raw token script under its stem,
// and replaying log.json against a Config rebuilt from match.json — tokens
// recompiled from that script — reproduces the recorded head with the token
// minted for real. Without the recorded tokens the replay cannot reproduce
// the log at all (the second run inside), which is what makes the capture
// load-bearing rather than decorative.
func TestFeedbackReplaysAMatchThatCreatesTokens(t *testing.T) {
	tok := tokenFixture(t)
	land := inlineCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	spell := inlineCard(t, "Name:Muster the Whelps\nManaCost:R\nTypes:Instant\nOracle:x\n"+
		"A:SP$ Token | TokenAmount$ 2 | TokenScript$ r_1_1_goblin | TokenOwner$ You | SpellDescription$ Create two 1/1 red Goblin tokens.\n")
	byName := map[string]*cards.Card{land.Faces[0].Name: land, spell.Faces[0].Name: spell}
	deckCards := make([]*cards.Card, 0, 40)
	for i := 0; i < 17; i++ {
		deckCards = append(deckCards, land)
	}
	for i := 0; i < 23; i++ {
		deckCards = append(deckCards, spell)
	}

	gate := &gateSeat{bot: seat.NewBot(1), reached: make(chan struct{}), release: make(chan struct{})}
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			if name != "m" {
				return host.Deck{}, host.ErrNotFound
			}
			return host.Deck{Name: "m", Cards: deckCards}, nil
		},
		Seats: func(seatNames []string, seed uint64) []seat.Seat {
			out := make([]seat.Seat, len(seatNames))
			out[0] = gate
			for i := 1; i < len(seatNames); i++ {
				out[i] = seat.NewBot(seed ^ uint64(i+1))
			}
			return out
		},
		Sleep:  func(time.Duration, <-chan struct{}) {},
		Tokens: map[string]*cards.Card{"r_1_1_goblin": tok},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "tok", Name: "Token table", Seats: 2,
		Decks: []string{"m", "m"}, Seed: 42, Spectator: view.Public}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("tok"); err != nil {
		t.Fatal(err)
	}

	// Advance seat 0's decisions until a token has actually been created —
	// by either seat's bot casting the muster — bounded, and deterministic
	// end to end (fixed seed, fixed decks, deterministic bots).
	stem := ""
	for i := 0; i < 60; i++ {
		select {
		case <-gate.reached:
		case <-time.After(30 * time.Second):
			t.Fatal("seat 0 never reached a decision")
		}
		gate.release <- struct{}{}
		snap, err := r.SnapshotForFeedback("tok", nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range snap.Log.Events {
			if ev.Kind == events.TokenCreate {
				stem = ev.Text
				break
			}
		}
		if stem != "" {
			break
		}
	}
	if stem == "" {
		t.Fatal("60 seat-0 decisions produced no TokenCreate — the deck or the bot policy changed")
	}
	if stem != "r_1_1_goblin" {
		t.Fatalf("first TokenCreate names %q, want r_1_1_goblin", stem)
	}

	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	rec := postFeedbackSeat(t, fs, "tokens look wrong", "http://localhost:8080/t/tok?seat=0", "")
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
	matchRaw, logRaw, _ := feedbackSnapshotFiles(t, dirs[0])

	var m feedbackMatchJSON
	if err := json.Unmarshal(matchRaw, &m); err != nil {
		t.Fatal(err)
	}
	src, ok := m.Tokens["r_1_1_goblin"]
	if !ok {
		t.Fatalf("match.json carries no script for the stem the log creates (%q); keys: %d", stem, len(m.Tokens))
	}
	if !strings.Contains(src, "Name:Goblin Token") || !strings.Contains(src, "PT:1/1") {
		t.Fatalf("recorded token script is not the fixture's: %q", src)
	}
	if strings.Contains(string(matchRaw), "tokens_unread") {
		t.Fatal("match.json records unread token scripts — the fixture's script file should read back")
	}
	var lg feedbackLog
	if err := json.Unmarshal(logRaw, &lg); err != nil {
		t.Fatal(err)
	}

	cfg := rebuildConfig(t, m, byName)
	e, err := replay.Replay(&lg.Log, cfg)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := e.L.Head(); got != lg.Head {
		t.Fatalf("replayed head %q, recorded %q", got, lg.Head)
	}
	n := 0
	for i := 1; i < len(e.G.Objs); i++ {
		o := &e.G.Objs[i]
		if o.IsToken && o.Face().Name == "Goblin Token" {
			n++
		}
	}
	if n == 0 {
		t.Fatal("the replayed game minted no Goblin Token — the token script was recorded but never minted")
	}
	t.Logf("%d Goblin Token objects in the replayed game at head %s", n, lg.Head)

	// The negative control: strip the recorded tokens and the replay must
	// fail — the engine's effToken emits an "unknown token script" Note
	// where the log records a TokenCreate, so the event streams part ways.
	// This is what makes match.json's tokens field load-bearing: a consumer
	// that skipped it does not get a silently token-less board, it gets an
	// error.
	cfgNoTokens := cfg
	cfgNoTokens.Tokens = nil
	if _, err := replay.Replay(&lg.Log, cfgNoTokens); err == nil {
		t.Fatal("replay without the recorded tokens reproduced the log — the token capture is not load-bearing")
	} else {
		t.Logf("replay without tokens, as expected: %v", err)
	}

	// A match keeps the compiled token after startup, so deleting the script
	// does not stop live play. It does make a later snapshot non-replayable if
	// this already-recorded TokenCreate is reached. The files and report must
	// still be retained, but report.json must say partial — never captured.
	if err := os.Remove(tok.Path); err != nil {
		t.Fatalf("remove live token's source script: %v", err)
	}
	unreadStore, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	rec = postFeedbackSeat(t, unreadStore, "token source disappeared", "http://localhost:8080/t/tok", "0")
	if rec.Code != http.StatusCreated {
		t.Fatalf("unread token: status = %d (body %q)", rec.Code, rec.Body.String())
	}
	unreadDirs := storedReports(t, unreadStore)
	if len(unreadDirs) != 1 {
		t.Fatalf("unread token: stored %d reports", len(unreadDirs))
	}
	status := readReport(t, unreadDirs[0]).Snapshot
	if !strings.HasPrefix(status, "partial: token scripts unavailable: r_1_1_goblin:") {
		t.Fatalf("unread token snapshot status %q, want partial with the unread-token reason", status)
	}
	unreadMatchRaw, unreadLogRaw, _ := feedbackSnapshotFiles(t, unreadDirs[0])
	var unreadMatch feedbackMatchJSON
	if err := json.Unmarshal(unreadMatchRaw, &unreadMatch); err != nil {
		t.Fatal(err)
	}
	if len(unreadMatch.TokensUnread) != 1 || !strings.HasPrefix(unreadMatch.TokensUnread[0], "r_1_1_goblin:") {
		t.Fatalf("tokens_unread = %q, want the missing live token", unreadMatch.TokensUnread)
	}
	var unreadLog feedbackLog
	if err := json.Unmarshal(unreadLogRaw, &unreadLog); err != nil {
		t.Fatal(err)
	}
	created := false
	for _, ev := range unreadLog.Events {
		if ev.Kind == events.TokenCreate && ev.Text == "r_1_1_goblin" {
			created = true
			break
		}
	}
	if !created {
		t.Fatal("partial snapshot's log has no r_1_1_goblin TokenCreate — unread script was not replay-critical")
	}
}

// TestFeedbackTableIDFromRoutes pins the URL shapes the client actually
// reports: the live table route with its seat query, and the archived-match
// route /t/<id>/m/<n> the web router serves — both read the table id, and a
// /t/ that appears only in a query string, a deeper path's other segments,
// or an undecodable escape never mangles the id.
func TestFeedbackTableIDFromRoutes(t *testing.T) {
	cases := []struct{ url, want string }{
		{"http://localhost:8080/t/t1?seat=0", "t1"},
		{"http://localhost:8080/t/t1/m/1?seat=0", "t1"},
		{"http://localhost:8080/t/t1/m/1", "t1"},
		{"http://localhost:8080/t/t1", "t1"},
		{"http://localhost:8080/t/t1#frag", "t1"},
		{"http://localhost:8080/t/t%31", "t1"},
		{"http://localhost:8080/x/t/t1", "t1"},
		{"http://localhost:8080/", ""},
		{"http://localhost:8080/t/", ""},
		{"http://localhost:8080/att/t1", ""},
		{"http://localhost:8080/?x=/t/t1", ""},
		{"http://localhost:8080/t/t%zz", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := tableIDFromURL(c.url); got != c.want {
			t.Errorf("tableIDFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// TestFeedbackCapturesFromAnArchivedMatchRouteURL is the POST-level
// regression for the route shape the unit test above pins: a report whose
// url is an archived-match route (/t/<id>/m/<n>, which the web router
// serves) still snapshots the live table behind it.
func TestFeedbackCapturesFromAnArchivedMatchRouteURL(t *testing.T) {
	_, advance, r := gatedRegistry(t, 4)
	advance(2)
	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
	if err != nil {
		t.Fatal(err)
	}
	rec := postFeedback(t, fs, "the replay jumped", "http://localhost:8080/t/t1/m/1?seat=0", nil)
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
	matchRaw, _, _ := feedbackSnapshotFiles(t, dirs[0])
	var m feedbackMatchJSON
	if err := json.Unmarshal(matchRaw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Table != "t1" {
		t.Fatalf("match.json table %q, want t1 (the route's first segment after /t/)", m.Table)
	}
}

// TestFeedbackSeatSourcePrecedence pins where the reporting seat comes from:
// the explicit `seat` form field wins over the url's seat= query parameter
// (valid or garbage), the query parameter is read when no field names a
// seat, and with neither the report still captures match/log but no view.
// An unparseable query seat is a recorded reason, never a 400.
func TestFeedbackSeatSourcePrecedence(t *testing.T) {
	_, advance, r := gatedRegistry(t, 4)
	advance(1)
	readView := func(t *testing.T, dir string) view.View {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, "view.json"))
		if err != nil {
			t.Fatalf("view.json: %v", err)
		}
		var v view.View
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	post := func(text, url, seat string) string {
		t.Helper()
		fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"), r)
		if err != nil {
			t.Fatal(err)
		}
		var rec *httptest.ResponseRecorder
		if seat == "" {
			rec = postFeedback(t, fs, text, url, nil)
		} else {
			rec = postFeedbackSeat(t, fs, text, url, seat)
		}
		if rec.Code != 201 {
			t.Fatalf("%s: status = %d (body %q)", text, rec.Code, rec.Body.String())
		}
		dirs := storedReports(t, fs)
		if len(dirs) != 1 {
			t.Fatalf("%s: stored %d reports", text, len(dirs))
		}
		return dirs[0]
	}

	// The explicit field beats the url's seat query.
	dir := post("field wins", "http://localhost:8080/t/t1?seat=0", "1")
	if got := readReport(t, dir).Snapshot; !strings.HasPrefix(got, "captured: match.json log.json view.json") {
		t.Fatalf("field-wins snapshot status %q", got)
	}
	if v := readView(t, dir); v.Viewer != 1 {
		t.Fatalf("viewer %d, want 1 — the explicit seat field, not the url's seat=0", v.Viewer)
	}

	// The field also beats a garbage url query.
	dir = post("field beats garbage", "http://localhost:8080/t/t1?seat=banana", "0")
	if v := readView(t, dir); v.Viewer != 0 {
		t.Fatalf("viewer %d, want 0 — the explicit field must win over the invalid query", v.Viewer)
	}

	// With no field, the url query is read.
	dir = post("query seat", "http://localhost:8080/t/t1?seat=2", "")
	if v := readView(t, dir); v.Viewer != 2 {
		t.Fatalf("viewer %d, want 2 — the url's seat query with no field", v.Viewer)
	}

	// With neither, the report captures match/log but no view.
	dir = post("no seat", "http://localhost:8080/t/t1", "")
	rep := readReport(t, dir)
	if got := rep.Snapshot; !strings.HasPrefix(got, "captured: match.json log.json") || strings.Contains(got, "view.json") {
		t.Fatalf("no-seat snapshot status %q, want captured without view.json", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "view.json")); !os.IsNotExist(err) {
		t.Errorf("view.json written for a report that named no seat")
	}

	// An unparseable query seat with no field is a recorded reason, not a
	// 400: the words survive.
	dir = post("garbage seat", "http://localhost:8080/t/t1?seat=banana", "")
	if got := readReport(t, dir).Snapshot; !strings.HasPrefix(got, "unavailable: seat") {
		t.Fatalf("garbage-query snapshot status %q, want unavailable: seat ...", got)
	}
}
