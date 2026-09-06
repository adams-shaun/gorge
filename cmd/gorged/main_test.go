package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func TestServesTablesOverHTTP(t *testing.T) {
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{cards: "../../.cards", decks: "../../internal/testutil/decks", tables: 1, seats: 2, pace: 0,
		cooldown: 0, dir: t.TempDir(), spectator: "omniscient", seed: 1, perpetual: false}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, ln) }()
	url := "http://" + ln.Addr().String()
	var tables []protocol.TableInfo
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&tables)
			resp.Body.Close()
			if len(tables) == 1 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no tables served: %v %+v", err, tables)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tables[0].ID != "t1" || tables[0].Seats != 2 || tables[0].Spectator != "omniscient" {
		t.Fatalf("%+v", tables[0])
	}
	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Fatalf("/: %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestHostThreadsCorpusTokens is the FL-40 required proof that gorged
// populates host.Options.Tokens from the opened corpus. A hosted table that
// does not get the token corpus mints no tokens and every token card is
// silently dead, so opening the corpus but dropping its Tokens onto the
// floor is exactly the failure this test is meant to catch.
func TestHostThreadsCorpusTokens(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := config{cards: "../../.cards", decks: "../../internal/testutil/decks", dir: t.TempDir()}
	opts := cfg.hostOptions(reg, deckLoader(reg, cfg.decks))
	if len(opts.Tokens) == 0 {
		t.Fatalf("host.Options.Tokens empty: token corpus not threaded from %s", cfg.cards)
	}
	if reg.Tokens == nil {
		t.Fatalf("corpus returned a nil Tokens map")
	}
}

func TestDeckDirectoryListingIsSortedAndComplete(t *testing.T) {
	names, err := deckFiles("../../internal/testutil/decks")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 12 || names[0] != "death-n-taxes" || names[len(names)-1] != "uw-tempo" {
		t.Fatalf("%v", names)
	}
}

// startServe boots serve on a random loopback port for the given config and
// returns its base URL, cancellation and error channel, mirroring the
// net.Listen("tcp", "127.0.0.1:0") pattern of TestServesTablesOverHTTP —
// no test below binds a fixed port. Every human-seat test here drives a
// real server with the corpus, because the seat-scoped routes are the whole
// point of the task; the corpus guard is the same one the existing tests
// use.
func startServe(t *testing.T, c config) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if c.cards == "" {
		c.cards = "../../.cards"
	}
	if c.decks == "" {
		c.decks = "../../internal/testutil/decks"
	}
	if c.dir == "" {
		c.dir = t.TempDir()
	}
	if c.spectator == "" {
		c.spectator = "omniscient"
	}
	if c.seed == 0 {
		c.seed = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, c, ln) }()
	return "http://" + ln.Addr().String(), cancel, done
}

// waitTables polls /api/tables until n tables are listed, so a request
// never races AddTable/StartAll on a freshly booted server.
func waitTables(t *testing.T, url string, n int) {
	t.Helper()
	var tables []protocol.TableInfo
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&tables)
			resp.Body.Close()
			if len(tables) == n {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %d tables served: %v %+v", n, err, tables)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// decisionOnce fetches one pending decision for a seat: status 200 with the
// decision, or the error status with a zero decision; a transport error is
// surfaced as status 0.
func decisionOnce(t *testing.T, pendingURL, seat, tok string) (decision.Decision, int) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s?seat=%s&token=%s", pendingURL, seat, tok))
	if err != nil {
		return decision.Decision{}, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decision.Decision{}, resp.StatusCode
	}
	var d decision.Decision
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("pending decode: %v", err)
	}
	return d, http.StatusOK
}

// waitDecision polls pending until a decision is actually offered.
func waitDecision(t *testing.T, pendingURL, seat, tok string) decision.Decision {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		d, status := decisionOnce(t, pendingURL, seat, tok)
		if status == http.StatusOK {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("no decision offered to seat %s: last status %d", seat, status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// seatViewOver reports whether match 1's seat-0 view is over, used as the
// "seat view changed" fallback when a game runs to completion instead of
// asking the seat again.
func seatViewOver(t *testing.T, url string) bool {
	t.Helper()
	resp, err := http.Get(url + "/api/tables/t1/matches/1/view?seat=0&token=tok")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var v view.View
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return false
	}
	return v.Over
}

// expectStatus polls path until the server answers with the wanted status
// (the only status this endpoint can answer once the server is up), then
// returns the decoded error body. Retrying makes the request immune to the
// server still booting.
func expectStatus(t *testing.T, url, path string, want int) protocol.ErrorBody {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + path)
		if err == nil {
			var eb protocol.ErrorBody
			_ = json.NewDecoder(resp.Body).Decode(&eb)
			resp.Body.Close()
			if resp.StatusCode == want {
				return eb
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: never answered %d (err %v)", path, want, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestHumanSeatTakesADecisionEndToEnd is the point of the task: a human
// seat answers the one decision a real person meets and the game visibly
// advances. serve runs with the -perpetual default (true) to prove R-E3-2
// forces t1 single-shot anyway, pace 0 so the wait is about the wire, and a
// fixed -seat-token so the test holds a seat without scraping stderr.
func TestHumanSeatTakesADecisionEndToEnd(t *testing.T) {
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, humansRaw: "0", pace: 0, perpetual: true, seatToken: "tok"})
	defer cancel()
	waitTables(t, url, 1)

	pendingURL := url + "/api/tables/t1/matches/1/pending"
	first := waitDecision(t, pendingURL, "0", "tok")

	// Answer with the first Min legal options of the decision actually on
	// offer — never a specific card name or option label; this test is
	// about the wire, not the game, and any Validate-accepted answer moves
	// the engine on.
	in := decision.Intent{Seq: first.Seq, Player: first.Player, Choices: make([]int, first.Min)}
	for i := range in.Choices {
		in.Choices[i] = i
	}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url+"/api/tables/t1/matches/1/intent", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	// The intent carries the token in the Authorization: Bearer header (the
	// reads use ?token=): R-E3-3 reads the header first, then the query.
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("intent answered %d, want 204", resp.StatusCode)
	}

	// The game must have advanced: the next decision asked of the seat has
	// a strictly higher Seq (the engine stamps Seq from the event count,
	// which only grows). If the match has run to completion instead — the
	// seat is never asked again — a view that is over is that same advance;
	// only a game frozen at seq %d is a failure.
	advanced := false
	deadline := time.Now().Add(15 * time.Second)
	for {
		d, status := decisionOnce(t, pendingURL, "0", "tok")
		if status == http.StatusOK && d.Seq > first.Seq {
			advanced = true
			break
		}
		if time.Now().After(deadline) {
			if seatViewOver(t, url) {
				advanced = true
				break
			}
			t.Fatalf("game did not advance past seq %d (last status %d)", first.Seq, status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !advanced {
		t.Fatal("game did not advance")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestHumanSeatRefusesTheOtherSeat: a token minted for seat 0 is refused
// when the request names seat 1 — the claim≠requested comparison of M2e-2,
// and the whole reason the resolver cannot be a rubber stamp (R-E3-3).
func TestHumanSeatRefusesTheOtherSeat(t *testing.T) {
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, humansRaw: "0", pace: 0, seatToken: "tok"})
	defer cancel()
	waitTables(t, url, 1)

	path := "/api/tables/t1/matches/1/pending?seat=1&token=tok"
	eb := expectStatus(t, url, path, http.StatusForbidden)
	if eb.Code != "forbidden" {
		t.Fatalf("code %q, want forbidden", eb.Code)
	}
	// The body names both seats — the claim's seat and the requested one.
	if !strings.Contains(eb.Message, "0") || !strings.Contains(eb.Message, "1") ||
		!strings.Contains(eb.Message, "not the requested seat") {
		t.Fatalf("message %q", eb.Message)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestHumanSeatRefusesAMissingToken: ?seat=0 with no token is a 401 — the
// resolver declines, and claimSeat refuses like an Authorize failure.
func TestHumanSeatRefusesAMissingToken(t *testing.T) {
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, humansRaw: "0", pace: 0, seatToken: "tok"})
	defer cancel()
	waitTables(t, url, 1)

	eb := expectStatus(t, url, "/api/tables/t1/matches/1/pending?seat=0", http.StatusUnauthorized)
	if eb.Code != "unauthorized" {
		t.Fatalf("code %q, want unauthorized", eb.Code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestHumanSeatRefusesAnUnknownToken: a token nobody was minted is a 401 —
// unknown tokens are refused exactly like absent ones (R-E3-3).
func TestHumanSeatRefusesAnUnknownToken(t *testing.T) {
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, humansRaw: "0", pace: 0, seatToken: "tok"})
	defer cancel()
	waitTables(t, url, 1)

	eb := expectStatus(t, url, "/api/tables/t1/matches/1/pending?seat=0&token=wrong", http.StatusUnauthorized)
	if eb.Code != "unauthorized" {
		t.Fatalf("code %q, want unauthorized", eb.Code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestBotTablesStayPerpetualBesideAHumanTable: with -tables 2 -humans 0 the
// -perpetual default (true) still applies to t2, while t1 is forced
// single-shot for its human (R-E3-2). Asserted on the TableConfig state
// directly, not by waiting out matches.
func TestBotTablesStayPerpetualBesideAHumanTable(t *testing.T) {
	c := config{seats: 2, tables: 2, seed: 1, perpetual: true, humansRaw: "0"}
	if err := c.applyHumans(); err != nil {
		t.Fatal(err)
	}
	cfgs := c.tableConfigs(nil, view.Omniscient)
	if len(cfgs) != 2 {
		t.Fatalf("want 2 table configs, got %d", len(cfgs))
	}
	t1, t2 := cfgs[0], cfgs[1]
	if t1.ID != "t1" || t1.Perpetual || len(t1.Humans) != 1 || t1.Humans[0] != 0 {
		t.Fatalf("t1 must be a single-shot human table: %+v", t1)
	}
	if t2.ID != "t2" || !t2.Perpetual || t2.Humans != nil {
		t.Fatalf("t2 must stay a perpetual bot table: %+v", t2)
	}
}

// TestNoHumansIsSpectatorOnly pins that without -humans the server behaves
// byte-identically to before the flag existed: a request naming a seat is
// refused 403, spectator-only, because Options.Seat stays nil.
func TestNoHumansIsSpectatorOnly(t *testing.T) {
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, pace: 0, perpetual: true})
	defer cancel()
	waitTables(t, url, 1)

	eb := expectStatus(t, url, "/api/tables/t1/matches/1/pending?seat=0", http.StatusForbidden)
	if eb.Code != "forbidden" || !strings.Contains(eb.Message, "spectator-only") {
		t.Fatalf("code %q message %q", eb.Code, eb.Message)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}
