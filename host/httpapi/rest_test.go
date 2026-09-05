package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func loader(t *testing.T) func(string) (host.Deck, error) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	by := map[string][]*cards.Card{}
	for i, n := range names {
		by[n] = decks[i]
	}
	return func(n string) (host.Deck, error) {
		cs, ok := by[n]
		if !ok {
			return host.Deck{}, host.ErrNotFound
		}
		return host.Deck{Name: n, Cards: cs}, nil
	}
}

// finishedServer plays one 4-seat match to completion and serves it.
func finishedServer(t *testing.T, o Options) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 5, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

func getJSON(t *testing.T, url string, into any) (int, protocol.ErrorBody) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%s: content-type %q", url, resp.Header.Get("Content-Type"))
	}
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		if err := json.Unmarshal(body, &e); err != nil {
			t.Fatalf("%s: non-JSON error body %q", url, body)
		}
		return resp.StatusCode, e
	}
	if into != nil {
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("%s: %v\n%s", url, err, body)
		}
	}
	return resp.StatusCode, protocol.ErrorBody{}
}

func postJSON(t *testing.T, url string, body any) (int, protocol.ErrorBody) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return resp.StatusCode, e
	}
	return resp.StatusCode, protocol.ErrorBody{}
}

func TestTablesAndMatches(t *testing.T) {
	srv, _ := finishedServer(t, Options{})
	var tables []protocol.TableInfo
	if code, _ := getJSON(t, srv.URL+"/api/tables", &tables); code != 200 || len(tables) != 1 || tables[0].ID != "t1" {
		t.Fatalf("%d %+v", code, tables)
	}
	var ms []protocol.MatchInfo
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches", &ms); code != 200 || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("%d %+v", code, ms)
	}
	if code, e := getJSON(t, srv.URL+"/api/tables/t9/matches", nil); code != 404 || e.Code != "not_found" {
		t.Fatalf("%d %+v", code, e)
	}
}

func TestViewAndEvents(t *testing.T) {
	srv, r := finishedServer(t, Options{})
	ms, _ := r.Matches("t1")
	head := uint64(ms[0].Events - 1)
	var v view.View
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/1/view", &v); code != 200 || !v.Over || v.Visibility != "omniscient" {
		t.Fatalf("view at head: %d %+v", code, v)
	}
	if code, _ := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/view?seq=%d", srv.URL, head/2), &v); code != 200 || v.Over {
		t.Fatalf("view mid: %d over=%v", code, v.Over)
	}
	code, e := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/view?seq=%d", srv.URL, head+1), nil)
	if code != 409 || e.Code != "beyond_head" || e.Head != head {
		t.Fatalf("beyond head: %d %+v", code, e)
	}
	if code, e := getJSON(t, srv.URL+"/api/tables/t1/matches/1/view?seq=abc", nil); code != 400 || e.Code != "bad_request" {
		t.Fatalf("bad seq: %d %+v", code, e)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/x/view", nil); code != 400 {
		t.Fatalf("bad k: %d", code)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/2/view", nil); code != 404 {
		t.Fatalf("unknown match: %d", code)
	}
	var evs []protocol.EventBody
	if code, _ := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=%d", srv.URL, head-2), &evs); code != 200 || len(evs) != 3 || evs[0].Event.Seq != head-2 {
		t.Fatalf("events tail: %d %+v", code, evs)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/1/events", &evs); code != 200 || len(evs) != int(head)+1 {
		t.Fatalf("all events: %d %d", code, len(evs))
	}
	if code, e := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=%d", srv.URL, head+1), nil); code != 409 || e.Head != head {
		t.Fatalf("events beyond head: %d %+v", code, e)
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	srv, r := finishedServer(t, Options{})
	s := r.OpenSession()
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "*", Mode: protocol.ModeOverview}); code != 204 {
		t.Fatalf("subscribe *: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t1", Mode: protocol.ModeFocus}); code != 204 {
		t.Fatalf("subscribe t1: %d", code)
	}
	if code, e := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: "s999", Table: "t1", Mode: protocol.ModeFocus}); code != 404 || e.Code != "not_found" {
		t.Fatalf("unknown session: %d %+v", code, e)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t1", Mode: "sideways"}); code != 400 {
		t.Fatalf("bad mode: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t9", Mode: protocol.ModeFocus}); code != 404 {
		t.Fatalf("unknown table: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/unsubscribe", protocol.Unsubscribe{Session: s.ID, Table: "t1"}); code != 204 {
		t.Fatalf("unsubscribe: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/unsubscribe", protocol.Unsubscribe{Session: s.ID, Table: "t1"}); code != 404 {
		t.Fatalf("unsubscribe twice: %d", code)
	}
	resp, _ := http.Post(srv.URL+"/api/subscribe", "application/json", bytes.NewReader([]byte("{")))
	if resp.StatusCode != 400 {
		t.Fatalf("malformed JSON: %d", resp.StatusCode)
	}
	resp, _ = http.Get(srv.URL + "/api/subscribe")
	if resp.StatusCode != 405 {
		t.Fatalf("GET subscribe: %d", resp.StatusCode)
	}
}

func TestAuthorizeGatesEveryRoute(t *testing.T) {
	denied := errors.New("no")
	srv, _ := finishedServer(t, Options{Authorize: func(r *http.Request) error {
		if r.Header.Get("X-Ok") == "1" {
			return nil
		}
		return denied
	}})
	for _, path := range []string{"/api/tables", "/api/tables/t1/matches", "/api/tables/t1/matches/1/view", "/api/tables/t1/matches/1/events"} {
		if code, e := getJSON(t, srv.URL+path, nil); code != 401 || e.Code != "unauthorized" {
			t.Fatalf("%s: %d %+v", path, code, e)
		}
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{}); code != 401 {
		t.Fatalf("subscribe: %d", code)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/tables", nil)
	req.Header.Set("X-Ok", "1")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("authorised request: %d", resp.StatusCode)
	}
}
