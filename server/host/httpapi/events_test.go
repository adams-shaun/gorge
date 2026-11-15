package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// statusErr is the canned error the fake backend returns; it is the shape
// eventhost's protocol errors take (status + code picked by the HTTP layer).
type statusErr struct {
	status int
	code   string
}

func (e statusErr) Error() string { return "event rejected: " + e.code }
func (e statusErr) Status() int   { return e.status }
func (e statusErr) Code() string  { return e.code }

// fakeEvents records the last call's method name, path id, token and decoded
// request, and answers with canned values or a canned error. lastMethod is
// the "was it called at all" probe: it stays "" until one method runs.
type fakeEvents struct {
	lastMethod string
	lastID     string
	lastToken  string
	lastReq    any

	err     error
	created protocol.CreateEventResponse
	state   protocol.EventState
	states  []protocol.EventState
	pack    protocol.DraftPack
	pool    protocol.EventPool
	check   protocol.DeckCheck
	log     protocol.EventLog
	sets    []protocol.EventSetInfo
}

func (f *fakeEvents) call(method, id, token string, req any) {
	f.lastMethod, f.lastID, f.lastToken, f.lastReq = method, id, token, req
}

func (f *fakeEvents) CreateEvent(req protocol.CreateEventRequest) (protocol.CreateEventResponse, error) {
	f.call("CreateEvent", "", "", req)
	if f.err != nil {
		return protocol.CreateEventResponse{}, f.err
	}
	return f.created, nil
}

func (f *fakeEvents) Events() []protocol.EventState {
	f.call("Events", "", "", nil)
	return f.states
}

func (f *fakeEvents) EventState(id, token string) (protocol.EventState, error) {
	f.call("EventState", id, token, nil)
	if f.err != nil {
		return protocol.EventState{}, f.err
	}
	return f.state, nil
}

func (f *fakeEvents) JoinEvent(id, token string, req protocol.JoinEventRequest) (protocol.EventState, error) {
	f.call("JoinEvent", id, token, req)
	if f.err != nil {
		return protocol.EventState{}, f.err
	}
	return f.state, nil
}

func (f *fakeEvents) DraftAdvice(id, token string) (protocol.DraftAdvice, error) {
	return protocol.DraftAdvice{Event: id, Waiting: true}, nil
}

func (f *fakeEvents) DraftPack(id, token string) (protocol.DraftPack, error) {
	f.call("DraftPack", id, token, nil)
	if f.err != nil {
		return protocol.DraftPack{}, f.err
	}
	return f.pack, nil
}

func (f *fakeEvents) Pick(id, token string, req protocol.PickRequest) (protocol.DraftPack, error) {
	f.call("Pick", id, token, req)
	if f.err != nil {
		return protocol.DraftPack{}, f.err
	}
	return f.pack, nil
}

func (f *fakeEvents) EventPool(id, token string) (protocol.EventPool, error) {
	f.call("EventPool", id, token, nil)
	if f.err != nil {
		return protocol.EventPool{}, f.err
	}
	return f.pool, nil
}

func (f *fakeEvents) CheckDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error) {
	f.call("CheckDeck", id, token, req)
	if f.err != nil {
		return protocol.DeckCheck{}, f.err
	}
	return f.check, nil
}

func (f *fakeEvents) SaveDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error) {
	f.call("SaveDeck", id, token, req)
	if f.err != nil {
		return protocol.DeckCheck{}, f.err
	}
	return f.check, nil
}

func (f *fakeEvents) Ready(id, token string, req protocol.ReadyRequest) (protocol.EventState, error) {
	f.call("Ready", id, token, req)
	if f.err != nil {
		return protocol.EventState{}, f.err
	}
	return f.state, nil
}

func (f *fakeEvents) Sideboard(id, token string, req protocol.SideboardRequest) (protocol.EventState, error) {
	f.call("Sideboard", id, token, req)
	if f.err != nil {
		return protocol.EventState{}, f.err
	}
	return f.state, nil
}

func (f *fakeEvents) EventLog(id, token string) (protocol.EventLog, error) {
	f.call("EventLog", id, token, nil)
	if f.err != nil {
		return protocol.EventLog{}, f.err
	}
	return f.log, nil
}

func (f *fakeEvents) SubscribeEvent(id, token, session string) error {
	f.call("SubscribeEvent", id, token, protocol.EventSubscribe{Session: session})
	return f.err
}

func (f *fakeEvents) Sets() []protocol.EventSetInfo {
	f.call("Sets", "", "", nil)
	if f.sets == nil {
		return []protocol.EventSetInfo{{Code: "FDN", Name: "Foundations", PackSize: 14, Ratings: "17lands"}}
	}
	return f.sets
}

func (f *fakeEvents) Personas() protocol.EventPersonas {
	f.call("Personas", "", "", nil)
	return protocol.EventPersonas{Drafters: []string{"rating-colour"}, Builders: []string{"best23"}}
}

// eventsServer builds a registry-backed handler with the given backend. The
// registry's LoadDeck is never exercised (no table is added), so a
// stub loader keeps this file corpus-free.
func eventsServer(t *testing.T, o Options) (*httptest.Server, *fakeEvents) {
	t.Helper()
	fake := &fakeEvents{
		state: protocol.EventState{Event: "e1", Set: "FDN", Phase: "LOBBY",
			Seats: []protocol.EventSeat{{Seat: 0, Kind: "human"}, {Seat: 1, Kind: "bot", Policy: "mucm"}}},
		created: protocol.CreateEventResponse{Event: "e1",
			Invites: []protocol.EventInvite{{Seat: 0, Token: "t0", Join: "/e/e1?seat=0&token=t0"}}},
		states: []protocol.EventState{{Event: "e1", Set: "FDN", Phase: "LOBBY"}},
		pack: protocol.DraftPack{Event: "e1", Seat: 2, Round: 1, PickNo: 3, Waiting: false,
			Pack: []protocol.DraftCard{{ID: "i7", Name: "Shock", Set: "FDN", Number: "140", Rarity: "C"}},
			Pool: []protocol.DraftCard{{ID: "i2", Name: "Llanowar Elves", Set: "FDN", Number: "222", Rarity: "C"}}},
		pool: protocol.EventPool{Event: "e1", Seat: 2, Rev: 1, Ready: false,
			Pool:       []protocol.DraftCard{{ID: "i2", Name: "Llanowar Elves", Set: "FDN", Number: "222", Rarity: "C"}},
			Suggestion: protocol.DeckRequest{Main: []string{"i2"}, Basics: [5]int{0, 4, 0, 0, 6}}},
		check: protocol.DeckCheck{Valid: true, Total: 40, AutoLands: [5]int{0, 2, 0, 0, 3}, Rev: 1},
		log: protocol.EventLog{Event: "e1", Complete: false,
			Entries: []json.RawMessage{json.RawMessage(`{"kind":"seat_joined"}`)}},
	}
	fake.states = append(fake.states, protocol.EventState{Event: "e2", Set: "FDN", Phase: "DRAFT"})
	r, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, host.ErrNotFound }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	o.Events = fake
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, fake
}

// doReq sends one request and returns status + raw body.
func doReq(t *testing.T, srv *httptest.Server, method, path, auth, body string) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw.Bytes()
}

// jsonEq asserts raw round-trips equal to want as JSON.
func jsonEq(t *testing.T, raw []byte, want any) {
	t.Helper()
	var got any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, raw)
	}
	wb, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var wv any
	if err := json.Unmarshal(wb, &wv); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wv) {
		t.Errorf("body mismatch:\n got %s\nwant %s", raw, wb)
	}
}

func bodyCode(t *testing.T, raw []byte) string {
	t.Helper()
	var eb protocol.ErrorBody
	if err := json.Unmarshal(raw, &eb); err != nil {
		t.Fatalf("body is not an ErrorBody: %v (%s)", err, raw)
	}
	return eb.Code
}

func TestEventRoutesOffAre404(t *testing.T) {
	srv, _ := eventsServerOff(t)
	st, raw := doReq(t, srv, "GET", "/api/events", "", "")
	if st != http.StatusNotFound {
		t.Errorf("GET /api/events off: status %d, want 404 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "not_found" {
		t.Errorf("GET /api/events off: code %q, want not_found", code)
	}
	st, raw = doReq(t, srv, "POST", "/api/events/e1/pick", `Bearer tok`, `{"round":1,"pick_no":1,"id":"i7"}`)
	if st != http.StatusNotFound {
		t.Errorf("POST pick off: status %d, want 404 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "not_found" {
		t.Errorf("POST pick off: code %q, want not_found", code)
	}
}

// eventsServerOff is eventsServer without a backend.
func eventsServerOff(t *testing.T) (*httptest.Server, *fakeEvents) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, host.ErrNotFound }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	srv := httptest.NewServer(NewHandler(r, Options{}))
	t.Cleanup(srv.Close)
	return srv, nil
}

func TestEventRoutesDispatch(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		path    string
		auth    string
		body    string
		call    string
		id      string
		token   string
		want    int
		req     any
		val     any
		raw     string // exact-body assertion instead of val
		nilList bool   // GET /api/events: clear the canned list first
	}{
		{
			name: "create", method: "POST", path: "/api/events", auth: "Bearer hosttok",
			body: `{"set":"FDN","seats":[{"kind":"human"},{"kind":"bot","policy":"mucm"},{"kind":"bot"},{"kind":"bot","drafter":"rating-bombs"}]}`,
			call: "CreateEvent", want: 200,
			req: protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
				{Kind: "human"}, {Kind: "bot", Policy: "mucm"}, {Kind: "bot"}, {Kind: "bot", Drafter: "rating-bombs"}}},
		},
		{name: "list", method: "GET", path: "/api/events", call: "Events", want: 200, raw: "[]", nilList: true},
		{name: "state", method: "GET", path: "/api/events/e1", auth: "Bearer seattok",
			call: "EventState", id: "e1", token: "seattok", want: 200,
			val: protocol.EventState{Event: "e1", Set: "FDN", Phase: "LOBBY",
				Seats: []protocol.EventSeat{{Seat: 0, Kind: "human"}, {Seat: 1, Kind: "bot", Policy: "mucm"}}}},
		{name: "join", method: "POST", path: "/api/events/e1/join", auth: "Bearer seattok", body: `{"name":"Ada"}`,
			call: "JoinEvent", id: "e1", token: "seattok", want: 200, req: protocol.JoinEventRequest{Name: "Ada"}},
		{name: "pack", method: "GET", path: "/api/events/e1/pack?token=qtok",
			call: "DraftPack", id: "e1", token: "qtok", want: 200,
			val: protocol.DraftPack{Event: "e1", Seat: 2, Round: 1, PickNo: 3,
				Pack: []protocol.DraftCard{{ID: "i7", Name: "Shock", Set: "FDN", Number: "140", Rarity: "C"}},
				Pool: []protocol.DraftCard{{ID: "i2", Name: "Llanowar Elves", Set: "FDN", Number: "222", Rarity: "C"}}}},
		{name: "pick", method: "POST", path: "/api/events/e1/pick", auth: "Bearer seattok",
			body: `{"round":1,"pick_no":3,"id":"i7"}`,
			call: "Pick", id: "e1", token: "seattok", want: 200,
			req: protocol.PickRequest{Round: 1, PickNo: 3, ID: "i7"}},
		{name: "pool", method: "GET", path: "/api/events/e1/pool", auth: "Bearer seattok",
			call: "EventPool", id: "e1", token: "seattok", want: 200,
			val: protocol.EventPool{Event: "e1", Seat: 2, Rev: 1,
				Pool:       []protocol.DraftCard{{ID: "i2", Name: "Llanowar Elves", Set: "FDN", Number: "222", Rarity: "C"}},
				Suggestion: protocol.DeckRequest{Main: []string{"i2"}, Basics: [5]int{0, 4, 0, 0, 6}}}},
		{name: "deck check", method: "POST", path: "/api/events/e1/deck/check", auth: "Bearer seattok",
			body: `{"main":["i2"],"basics":[3,0,0,0,2]}`,
			call: "CheckDeck", id: "e1", token: "seattok", want: 200,
			req: protocol.DeckRequest{Main: []string{"i2"}, Basics: [5]int{3, 0, 0, 0, 2}}},
		{name: "deck save", method: "POST", path: "/api/events/e1/deck", auth: "Bearer seattok",
			body: `{"main":["i2"],"basics":[3,0,0,0,2]}`,
			call: "SaveDeck", id: "e1", token: "seattok", want: 200,
			req: protocol.DeckRequest{Main: []string{"i2"}, Basics: [5]int{3, 0, 0, 0, 2}}},
		{name: "ready", method: "POST", path: "/api/events/e1/ready", auth: "Bearer seattok", body: `{"rev":1}`,
			call: "Ready", id: "e1", token: "seattok", want: 200, req: protocol.ReadyRequest{Rev: 1}},
		{name: "sideboard", method: "POST", path: "/api/events/e1/sideboard", auth: "Bearer seattok", body: `{"game":2,"keep":true}`,
			call: "Sideboard", id: "e1", token: "seattok", want: 200, req: protocol.SideboardRequest{Game: 2, Keep: true}},
		{name: "sideboard deck", method: "POST", path: "/api/events/e1/sideboard", auth: "Bearer seattok",
			body: `{"game":2,"deck":{"main":["i2"],"basics":[3,0,0,0,2]}}`,
			call: "Sideboard", id: "e1", token: "seattok", want: 200,
			req: protocol.SideboardRequest{Game: 2, Deck: &protocol.DeckRequest{Main: []string{"i2"}, Basics: [5]int{3, 0, 0, 0, 2}}}},
		{name: "log", method: "GET", path: "/api/events/e1/log", auth: "Bearer spectok",
			call: "EventLog", id: "e1", token: "spectok", want: 200,
			val: protocol.EventLog{Event: "e1", Complete: false,
				Entries: []json.RawMessage{json.RawMessage(`{"kind":"seat_joined"}`)}}},
		{name: "subscribe", method: "POST", path: "/api/events/e1/subscribe", auth: "Bearer seattok",
			body: `{"session":"s9"}`,
			call: "SubscribeEvent", id: "e1", token: "seattok", want: 204,
			req: protocol.EventSubscribe{Session: "s9"}},
		{name: "sets", method: "GET", path: "/api/events/sets",
			call: "Sets", want: 200,
			val: []protocol.EventSetInfo{{Code: "FDN", Name: "Foundations", PackSize: 14, Ratings: "17lands"}}},
		{name: "personas", method: "GET", path: "/api/events/personas",
			call: "Personas", want: 200,
			val: protocol.EventPersonas{Drafters: []string{"rating-colour"}, Builders: []string{"best23"}}},
	}
	srv, fake := eventsServer(t, Options{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake.lastMethod, fake.lastID, fake.lastToken, fake.lastReq = "", "", "", nil
			if tc.nilList {
				fake.states = nil
			}
			st, raw := doReq(t, srv, tc.method, tc.path, tc.auth, tc.body)
			if st != tc.want {
				t.Fatalf("status %d, want %d (body %s)", st, tc.want, raw)
			}
			if fake.lastMethod != tc.call {
				t.Errorf("backend method %q, want %q", fake.lastMethod, tc.call)
			}
			if fake.lastID != tc.id {
				t.Errorf("backend id %q, want %q", fake.lastID, tc.id)
			}
			if fake.lastToken != tc.token {
				t.Errorf("backend token %q, want %q", fake.lastToken, tc.token)
			}
			if tc.req != nil && !reflect.DeepEqual(fake.lastReq, tc.req) {
				t.Errorf("decoded request %#v, want %#v", fake.lastReq, tc.req)
			}
			switch {
			case tc.raw != "":
				if got := string(bytes.TrimRight(raw, "\n")); got != tc.raw {
					t.Errorf("body %q, want %q", got, tc.raw)
				}
			case tc.val != nil:
				jsonEq(t, raw, tc.val)
			}
		})
	}
}

// TestEventRoutesListNilIsEmpty is the nil → [] half of GET /api/events.
func TestEventRoutesListNilIsEmpty(t *testing.T) {
	srv, fake := eventsServer(t, Options{})
	fake.states = nil
	st, raw := doReq(t, srv, "GET", "/api/events", "", "")
	if st != http.StatusOK {
		t.Fatalf("status %d, want 200", st)
	}
	if got := string(bytes.TrimRight(raw, "\n")); got != "[]" {
		t.Errorf("body %q, want []", got)
	}
}

func TestEventTokenPrecedence(t *testing.T) {
	srv, fake := eventsServer(t, Options{})
	for _, tc := range []struct {
		name, path, auth, want string
	}{
		{"bearer wins over query", "/api/events/e1?token=qtok", "Bearer htok", "htok"},
		{"query alone", "/api/events/e1?token=qtok", "", "qtok"},
		{"neither", "/api/events/e1", "", ""},
		{"empty query", "/api/events/e1?token=", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake.lastToken = "sentinel"
			doReq(t, srv, "GET", tc.path, tc.auth, "")
			if fake.lastToken != tc.want {
				t.Errorf("token %q, want %q", fake.lastToken, tc.want)
			}
		})
	}
}

func TestEventErrorMapping(t *testing.T) {
	srv, fake := eventsServer(t, Options{})

	fake.err = statusErr{status: 409, code: "stale_pick"}
	st, raw := doReq(t, srv, "POST", "/api/events/e1/pick", "Bearer tok", `{"round":1,"pick_no":1,"id":"i7"}`)
	if st != http.StatusConflict {
		t.Errorf("stale_pick: status %d, want 409 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "stale_pick" {
		t.Errorf("stale_pick: code %q, want stale_pick", code)
	}

	fake.err = statusErr{status: 422, code: "invalid_deck"}
	st, raw = doReq(t, srv, "POST", "/api/events/e1/deck", "Bearer tok", `{"main":["i2"],"basics":[0,0,0,0,0]}`)
	if st != http.StatusUnprocessableEntity {
		t.Errorf("invalid_deck: status %d, want 422 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "invalid_deck" {
		t.Errorf("invalid_deck: code %q, want invalid_deck", code)
	}

	fake.err = errors.New("boom")
	st, raw = doReq(t, srv, "GET", "/api/events/e1/pack", "Bearer tok", "")
	if st != http.StatusInternalServerError {
		t.Errorf("plain error: status %d, want 500 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "internal" {
		t.Errorf("plain error: code %q, want internal", code)
	}
}

func TestEventRoutesWrongMethod(t *testing.T) {
	srv, fake := eventsServer(t, Options{})
	fake.lastMethod = "sentinel"
	st, raw := doReq(t, srv, "PUT", "/api/events/e1/pick", "Bearer tok", `{"round":1,"pick_no":1,"id":"i7"}`)
	if st != http.StatusMethodNotAllowed {
		t.Errorf("PUT pick: status %d, want 405 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "method_not_allowed" {
		t.Errorf("PUT pick: code %q, want method_not_allowed", code)
	}
	if fake.lastMethod != "sentinel" {
		t.Errorf("backend called with method %q; the twin must answer before dispatch", fake.lastMethod)
	}
	fake.lastMethod = "sentinel"
	st, raw = doReq(t, srv, "GET", "/api/events/e1/sideboard", "Bearer tok", "")
	if st != http.StatusMethodNotAllowed {
		t.Errorf("GET sideboard: status %d, want 405 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "method_not_allowed" {
		t.Errorf("GET sideboard: code %q, want method_not_allowed", code)
	}
	if fake.lastMethod != "sentinel" {
		t.Errorf("sideboard twin dispatched to the backend (%s)", fake.lastMethod)
	}
}

func TestEventMalformedBody(t *testing.T) {
	srv, fake := eventsServer(t, Options{})
	st, raw := doReq(t, srv, "POST", "/api/events/e1/pick", "Bearer tok", `{"x":1}`)
	if st != http.StatusBadRequest {
		t.Errorf("malformed pick body: status %d, want 400 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "bad_request" {
		t.Errorf("malformed pick body: code %q, want bad_request", code)
	}
	if fake.lastMethod != "" {
		t.Errorf("backend was called (%s) on a malformed body; decode must precede dispatch", fake.lastMethod)
	}
	st, raw = doReq(t, srv, "POST", "/api/events/e1/sideboard", "Bearer tok", `{"x":1}`)
	if st != http.StatusBadRequest {
		t.Errorf("malformed sideboard body: status %d, want 400 (body %s)", st, raw)
	}
	if code := bodyCode(t, raw); code != "bad_request" {
		t.Errorf("malformed sideboard body: code %q, want bad_request", code)
	}
	if fake.lastMethod != "" {
		t.Errorf("sideboard backend was called (%s) on a malformed body; decode must precede dispatch", fake.lastMethod)
	}
}
