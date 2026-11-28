package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestDeckCheckRoute pins the handler's half of POST /api/decks/check: off
// without CheckDeck, exactly one of deck and text, the builder's report
// answered verbatim, a JSON 405 for another method, and a builder's
// DeckRefused on POST /api/games answered 400 deck_refused with its report.
func TestDeckCheckRoute(t *testing.T) {
	reg, err := host.New(host.Options{
		LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, errors.New("no decks") },
		Sleep:    func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	stub := protocol.DeckReport{Name: "n", Format: "commander", Engine: "gorge", Cards: 100,
		Commanders: []string{"Ghoulcaller Gisa"}, Identity: "B", Problems: []string{"p"}, Unknown: []string{"Sol Rnig"},
		Unsupported: []protocol.DeckReportCard{{Name: "Some Card", Count: 1, Needs: []string{"X"}}},
		Substituted: []protocol.DeckSubstitution{}}
	var calls int
	on := httptest.NewServer(NewHandler(reg, Options{
		CheckDeck: func(DeckCheckRequest) (protocol.DeckReport, error) { calls++; return stub, nil },
		CreateGame: func(CreateGameOptions) (CreateGameResponse, error) {
			return CreateGameResponse{}, DeckRefused{Report: stub, Msg: "seat 0 deck: refused"}
		},
	}))
	t.Cleanup(on.Close)
	off := httptest.NewServer(NewHandler(reg, Options{}))
	t.Cleanup(off.Close)

	do := func(url, method, path, body string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, url+path, bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, buf.Bytes()
	}
	errCode := func(raw []byte) protocol.ErrorBody {
		var e protocol.ErrorBody
		_ = json.Unmarshal(raw, &e)
		return e
	}

	if code, raw := do(off.URL, "POST", "/api/decks/check", `{"text":"1 Sol Ring"}`); code != http.StatusNotFound {
		t.Errorf("no CheckDeck: %d %s, want 404", code, raw)
	}
	for name, body := range map[string]string{
		"deck and text": `{"text":"1 Sol Ring","deck":{"name":"x"}}`,
		"neither":       `{"format":"commander"}`,
		"bad format":    `{"format":"modern","text":"1 Sol Ring"}`,
		"malformed":     `{"text":`,
	} {
		if code, raw := do(on.URL, "POST", "/api/decks/check", body); code != http.StatusBadRequest || errCode(raw).Code != "bad_request" {
			t.Errorf("%s: %d %s, want 400 bad_request", name, code, raw)
		}
	}
	if calls != 0 {
		t.Fatalf("CheckDeck ran %d times on refused requests", calls)
	}
	code, raw := do(on.URL, "POST", "/api/decks/check", `{"text":"1 Sol Ring"}`)
	var got protocol.DeckReport
	if err := json.Unmarshal(raw, &got); code != http.StatusOK || err != nil || !reflect.DeepEqual(got, stub) {
		t.Errorf("check: %d %s (%v), want 200 with the stub report %+v", code, raw, err, stub)
	}
	if calls != 1 {
		t.Errorf("CheckDeck ran %d times, want 1", calls)
	}
	if code, raw := do(on.URL, "GET", "/api/decks/check", ""); code != http.StatusMethodNotAllowed || errCode(raw).Code != "method_not_allowed" {
		t.Errorf("GET: %d %s, want 405 method_not_allowed", code, raw)
	}

	code, raw = do(on.URL, "POST", "/api/games", `{"format":"commander","seats":4,"humans":1,"decks":[{"seat":0,"text":"1 Sol Ring"}]}`)
	e := errCode(raw)
	if code != http.StatusBadRequest || e.Code != "deck_refused" || e.Message != "seat 0 deck: refused" || e.Deck == nil || !reflect.DeepEqual(*e.Deck, stub) {
		t.Errorf("refused deck on POST /api/games: %d %s, want 400 deck_refused with the report", code, raw)
	}
}
