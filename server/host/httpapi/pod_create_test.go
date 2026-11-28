package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// podServer arms CreateGame with a recorder, so a test reads exactly what the
// handler validated and handed across the seam.
func podServer(t *testing.T, got *CreateGameOptions) *httptest.Server {
	t.Helper()
	r, rerr := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if rerr != nil {
		t.Fatal(rerr)
	}
	t.Cleanup(func() { r.Close() })
	srv := httptest.NewServer(NewHandler(r, Options{
		CreateGame: func(o CreateGameOptions) (CreateGameResponse, error) {
			*got = o
			return CreateGameResponse{Table: "pod", Match: 1, Seat: 0, Token: "t0",
				Seats: []SeatGrant{{Seat: 0, Token: "t0", Join: "/t/pod?seat=0&token=t0"}}}, nil
		},
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCreatePodRequestIsValidatedAtTheHandler(t *testing.T) {
	var got CreateGameOptions
	srv := podServer(t, &got)

	ok, err, raw := postGames(t, srv.URL, `{"format":"commander","seats":4,"humans":2,"decks":[{"seat":1,"deck":{"name":"x","cards":[]}}]}`)
	if ok != http.StatusOK {
		t.Fatalf("pod request: %d %+v %s", ok, err, raw)
	}
	if got.Format != host.FormatCommander || got.Seats != 4 || got.Humans != 2 || len(got.Decks) != 1 || got.Decks[0].Seat != 1 || !strings.Contains(string(got.Decks[0].Deck), `"name":"x"`) {
		t.Fatalf("options handed to CreateGame = %+v", got)
	}
	// An omitted seats/humans keeps the original 2-seat, one-human table.
	if code, e, raw := postGames(t, srv.URL, `{"format":"constructed"}`); code != http.StatusOK {
		t.Fatalf("plain request: %d %+v %s", code, e, raw)
	}
	if got.Seats != 2 || got.Humans != 1 || len(got.Decks) != 0 {
		t.Fatalf("defaults = seats %d humans %d decks %d, want 2/1/0", got.Seats, got.Humans, len(got.Decks))
	}

	for _, tc := range []struct{ name, body, want string }{
		{"one seat", `{"format":"commander","seats":1}`, "seats"},
		{"five seats", `{"format":"commander","seats":5}`, "seats"},
		{"constructed pod", `{"format":"constructed","seats":4}`, "commander"},
		{"no human", `{"format":"commander","seats":4,"humans":-1}`, "humans"},
		{"more humans than seats", `{"format":"commander","seats":2,"humans":3}`, "humans"},
		{"deck seat out of range", `{"format":"commander","seats":4,"decks":[{"seat":4,"deck":{}}]}`, "seat 4"},
		{"deck seat twice", `{"format":"commander","seats":4,"decks":[{"seat":1,"deck":{}},{"seat":1,"deck":{}}]}`, "twice"},
		{"deck missing", `{"format":"commander","seats":4,"decks":[{"seat":1}]}`, "deck"},
	} {
		code, e, raw := postGames(t, srv.URL, tc.body)
		if code != http.StatusBadRequest || !strings.Contains(e.Message, tc.want) {
			t.Errorf("%s: %d %q (%s), want 400 naming %q", tc.name, code, e.Message, raw, tc.want)
		}
	}
}

// TestCreateGameResponseCarriesDeckReports: a successful POST /api/games
// hands back the per-seat deck reports beside the table identity, so a
// substitution is never silent; an empty list stays omitted.
func TestCreateGameResponseCarriesDeckReports(t *testing.T) {
	resp := CreateGameResponse{
		Table: "pod", Match: 1, Seed: 7, Seat: 0, Token: "t0", BotPolicy: "bot",
		Join: "/t/pod?seat=0&token=t0",
		Decks: []protocol.DeckReport{{
			Name: "seat 0 deck", Format: "commander", Engine: "gorge", Cards: 100,
			Commanders: []string{}, Problems: []string{}, Unknown: []string{},
			Unsupported: []protocol.DeckReportCard{},
			Substituted: []protocol.DeckSubstitution{{Out: "Unplayable Card", In: "Swamp", Count: 2}},
		}},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"decks":[`) {
		t.Fatalf("marshalled response has no decks: %s", raw)
	}
	var got struct {
		Decks []protocol.DeckReport `json:"decks"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Decks) != 1 || len(got.Decks[0].Substituted) != 1 ||
		got.Decks[0].Substituted[0].Out != "Unplayable Card" || got.Decks[0].Substituted[0].In != "Swamp" ||
		got.Decks[0].Engine != "gorge" {
		t.Fatalf("decks round trip = %+v", got.Decks)
	}

	empty, err := json.Marshal(CreateGameResponse{Table: "pod"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(empty), `"decks"`) {
		t.Fatalf("a response with no submitted seats carries decks: %s", empty)
	}
}
