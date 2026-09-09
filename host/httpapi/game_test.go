package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// gameCreator builds the Options.CreateGame closure the endpoint needs, in
// the exact shape cmd/gorged wires: a fresh single-shot 2-seat table with
// seat 0 human (driven from outside) and seat 1 a bot, dealt from the
// caller-supplied pool, added to the registry and started. The returned
// token is the human seat's bearer credential; the join path carries it.
func gameCreator(r *host.Registry, pool []string) func(host.Format) (CreateGameResponse, error) {
	return func(gf host.Format) (CreateGameResponse, error) {
		id := host.TableID("g1")
		cfg := host.TableConfig{ID: id, Name: "Play vs bot", Seats: 2, Decks: pool,
			Seed: 7, Spectator: view.Omniscient, Perpetual: false, Humans: []int{0}, Format: gf}
		if err := r.AddTable(cfg); err != nil {
			return CreateGameResponse{}, err
		}
		if err := r.Start(id); err != nil {
			return CreateGameResponse{}, err
		}
		return CreateGameResponse{Table: "g1", Match: 1, Seed: 7, Seat: 0, Token: "tok",
			Join: "/t/g1?seat=0&token=tok"}, nil
	}
}

// gamesServer builds a registry + handler with a CreateGame armed.
func gamesServer(t *testing.T, o Options) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	o.CreateGame = gameCreator(r, []string{"a", "b"})
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

func postGames(t *testing.T, url, body string) (int, protocol.ErrorBody, []byte) {
	t.Helper()
	resp, err := http.Post(url+"/api/games", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		_ = json.Unmarshal(raw.Bytes(), &e)
		return resp.StatusCode, e, raw.Bytes()
	}
	return resp.StatusCode, protocol.ErrorBody{}, raw.Bytes()
}

// TestCreateGameStartsARealHumanVsBotMatch is the task's "starts a real
// match" proof: POST /api/games returns a join identity, and the table it
// created goes live and parks the human seat on a real decision — the
// human-vs-bot concept, expressed end to end over the wire.
func TestCreateGameStartsARealHumanVsBotMatch(t *testing.T) {
	srv, r := gamesServer(t, Options{})
	status, _, raw := postGames(t, srv.URL, `{"format":"constructed"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d body %s", status, raw)
	}
	var resp CreateGameResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Table != "g1" || resp.Seat != 0 || resp.Token != "tok" || resp.Seed != 7 {
		t.Fatalf("response %+v", resp)
	}
	if resp.Join != "/t/g1?seat=0&token=tok" {
		t.Fatalf("join %q", resp.Join)
	}
	// The table is live and the human seat is parked on a decision it must
	// answer — a real match, not a config that merely registered.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("no human decision parked after POST /api/games")
		}
		if _, err := r.Pending("g1", 1, 0); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
}

// TestCreateGameRejectsUnknownFormat ensures a typo'd format is a 400 with a
// readable body, and an absent format defaults to constructed (the wire's
// zero value, not an error).
func TestCreateGameRejectsUnknownFormat(t *testing.T) {
	srv, _ := gamesServer(t, Options{})
	status, e, _ := postGames(t, srv.URL, `{"format":"modern"}`)
	if status != http.StatusBadRequest || e.Code != "bad_request" {
		t.Fatalf("status %d err %+v, want 400 bad_request", status, e)
	}
	status, _, raw := postGames(t, srv.URL, `{}`)
	if status != http.StatusOK {
		t.Fatalf("empty format status %d body %s", status, raw)
	}
}

// TestCreateGameDisabledWithoutBuilder covers the server that never wired the
// flow: POST /api/games answers 404 like any unserved path.
func TestCreateGameDisabledWithoutBuilder(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	srv := httptest.NewServer(NewHandler(r, Options{}))
	t.Cleanup(srv.Close)
	status, e, _ := postGames(t, srv.URL, `{"format":"constructed"}`)
	if status != http.StatusNotFound || e.Code != "not_found" {
		t.Fatalf("status %d err %+v, want 404 not_found", status, e)
	}
}
