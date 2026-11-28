package gorged

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// restartServer is a gorge server dealing constructed pool deck "p1" and
// Commander pool deck "c1", with what a test reads off it.
type restartServer struct {
	url     string
	imports *deckImports
	reg     *host.Registry
}

func newRestartServer(t *testing.T) restartServer {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cmdrName := testutil.CommanderAcceptanceDeckNames()[0]
	cmdrFile := testutil.RepoDeckFile(t, cmdrName)
	cmdrCards := testutil.RepoDeck(t, reg, cmdrName)
	conCards := testutil.RepoDeck(t, reg, "ur-delver")
	base := func(id string) (host.Deck, error) {
		switch id {
		case "p1":
			return host.Deck{Name: id, Cards: conCards}, nil
		case "c1":
			return host.Deck{Name: id, Cards: cmdrCards, Commanders: []int{cmdrFile.CommanderIndex()}}, nil
		}
		return host.Deck{}, host.ErrNotFound
	}
	c := config{imports: newDeckImports(reg)}
	r, err := host.New(host.Options{LoadDeck: c.imports.loader(base), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	gate := &seatGate{tokenToClaim: map[string]httpapi.SeatClaim{}, claimTokens: map[httpapi.SeatClaim]string{}}
	srv := httptest.NewServer(httpapi.NewHandler(r, httpapi.Options{
		CreateGame: c.createGame(r, gate, []string{"c1"}, []string{"p1"}, view.Public),
		Seat:       gate.resolve,
	}))
	t.Cleanup(srv.Close)
	return restartServer{url: srv.URL, imports: c.imports, reg: r}
}

// create posts body and returns the new table and the id its seat 0 plays.
func (s restartServer) create(t *testing.T, body map[string]any) (httpapi.CreateGameResponse, string) {
	t.Helper()
	code, raw := podPost(t, s.url, podBody(t, body))
	if code != http.StatusOK {
		t.Fatalf("POST /api/games %v: %d %s", body["format"], code, raw)
	}
	var resp httpapi.CreateGameResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	var ms []protocol.MatchInfo
	for deadline := time.Now().Add(testutil.WaitBudget); len(ms) == 0 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		ms, _ = s.reg.Matches(host.TableID(resp.Table))
	}
	if len(ms) != 1 || len(ms[0].Seats) != 2 {
		t.Fatalf("table %q matches: %+v", resp.Table, ms)
	}
	return resp, ms[0].Seats[0].DeckID
}

// refused posts body and asserts a 400 naming want that adds no table.
func (s restartServer) refused(t *testing.T, body map[string]any, want string) {
	t.Helper()
	before := len(s.reg.Tables())
	code, raw := podPost(t, s.url, podBody(t, body))
	var msg struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if code != http.StatusBadRequest || !strings.Contains(msg.Message, want) {
		t.Errorf("POST /api/games format %v humans %v human_deck %v bot_deck %v: %d %.200s, want 400 %q",
			body["format"], body["humans"], body["human_deck"], body["bot_deck"], code, raw, want)
	}
	if after := len(s.reg.Tables()); after != before {
		t.Errorf("a refused request left %d tables, want %d", after, before)
	}
}

// TestRestartDeckIDIsNotGuessable: a client that knows only another player's
// table id cannot build the id that seats that player's submitted deck. The
// table is private, so the id is readable only with its seat claim.
func TestRestartDeckIDIsNotGuessable(t *testing.T) {
	s := newRestartServer(t)
	text := constructedText(testutil.RepoDeckFile(t, "ur-delver"))
	body := map[string]any{"format": "constructed", "decks": []map[string]any{{"seat": 0, "text": text}}}
	victim, id := s.create(t, body)
	_, other := s.create(t, body)

	resp, err := http.Get(s.url + "/api/tables/" + victim.Table + "/matches")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a stranger reads the victim table's matches: %d, want 404", resp.StatusCode)
	}
	if _, err := s.imports.loader(func(string) (host.Deck, error) { return host.Deck{}, host.ErrNotFound })(id); err != nil {
		t.Fatalf("the victim's deck %q is not stored: %v", id, err)
	}
	guess := importDeckID(host.TableID(victim.Table), 0)
	suffix, ok := strings.CutPrefix(id, guess+"-")
	if !ok || len(suffix) < 32 {
		t.Errorf("seat 0 deck id %q, want %q followed by a random suffix", id, guess)
	}
	if otherSuffix := other[strings.LastIndex(other, "-")+1:]; otherSuffix == suffix {
		t.Errorf("two tables share the suffix %q", suffix)
	}
	for _, req := range []map[string]any{
		{"format": "constructed", "human_deck": guess, "bot_deck": "p1"},
		{"format": "constructed", "humans": 2, "human_deck": "p1", "bot_deck": guess},
	} {
		want := "unknown human deck"
		if req["human_deck"] == "p1" {
			want = "unknown bot deck"
		}
		s.refused(t, req, want)
	}
}

// TestRestartKeepsTheSubmittedDecksFormat: a carried deck was checked for its
// first table's format, so a rematch in the other format is refused, in the
// words a pool deck of the wrong format gets.
func TestRestartKeepsTheSubmittedDecksFormat(t *testing.T) {
	s := newRestartServer(t)
	decks := map[string]string{
		"constructed": constructedText(testutil.RepoDeckFile(t, "ur-delver")),
		"commander":   commanderText(testutil.RepoDeckFile(t, testutil.CommanderAcceptanceDeckNames()[0])),
	}
	for _, tc := range []struct{ from, to, toPool string }{
		{"commander", "constructed", "p1"},
		{"constructed", "commander", "c1"},
	} {
		_, id := s.create(t, map[string]any{"format": tc.from, "decks": []map[string]any{{"seat": 0, "text": decks[tc.from]}}})
		if !strings.HasPrefix(id, "import-") {
			t.Fatalf("%s table seat 0 plays %q, want a submitted deck", tc.from, id)
		}
		stored := len(s.imports.decks)
		want := "belongs to " + tc.from + ", not requested format " + tc.to
		s.refused(t, map[string]any{"format": tc.to, "human_deck": id, "bot_deck": tc.toPool}, "human deck \""+id+"\" "+want)
		s.refused(t, map[string]any{"format": tc.to, "human_deck": tc.toPool, "bot_deck": id}, "bot deck \""+id+"\" "+want)
		if got := len(s.imports.decks); got != stored {
			t.Errorf("a refused rematch left %d stored decks, want %d", got, stored)
		}
		// The same pick at its own format is accepted.
		s.create(t, map[string]any{"format": tc.from, "human_deck": id, "bot_deck": map[string]string{"constructed": "p1", "commander": "c1"}[tc.from]})
	}
}

// TestReapReleasesSealedDeckIDs: a reaped table's sealed ids and their restart
// marks go with it, and a table whose id merely starts the same keeps its own.
func TestReapReleasesSealedDeckIDs(t *testing.T) {
	imp := newDeckImports(nil)
	seal := func(table host.TableID) string {
		t.Helper()
		ids := []string{importDeckID(table, 0), ""}
		want := []string{ids[0], "p1"}
		if err := sealImportIDs(want, ids); err != nil {
			t.Fatal(err)
		}
		if ids[0] == importDeckID(table, 0) || want[0] != ids[0] || ids[1] != "" || want[1] != "p1" {
			t.Fatalf("sealed ids %q, want %q", ids, want)
		}
		if err := imp.putAll(ids, map[int]host.Deck{0: {Name: string(table)}}); err != nil {
			t.Fatal(err)
		}
		imp.markRestartable(ids, host.FormatConstructed)
		if _, _, ok := imp.heldForRestart(ids[0]); !ok {
			t.Fatalf("%q is not held for restart", ids[0])
		}
		return ids[0]
	}
	gone, kept := seal("g1"), seal("g10")

	imp.dropTable("g1")

	if _, ok := imp.decks[gone]; ok {
		t.Errorf("%q still stored after g1 was reaped", gone)
	}
	if _, ok := imp.restartable[gone]; ok {
		t.Errorf("%q still marked restartable after g1 was reaped", gone)
	}
	if _, _, ok := imp.heldForRestart(kept); !ok {
		t.Errorf("%q dropped by g1's reap", kept)
	}
}
