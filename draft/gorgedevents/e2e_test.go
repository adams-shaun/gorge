package gorgedevents_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/eventhost"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mapGate is the seat gate the e2e stands in for cmd/gorged's seatGate: a
// mint/resolve pair over two maps, reading the bearer header and the ?token=
// query exactly like the production gate, so the event game tokens the
// eventhost mints resolve through Options.Seat like any other seat claim.
type mapGate struct {
	mu     sync.Mutex
	next   int
	tokens map[string]httpapiSeatClaim
	claims map[httpapiSeatClaim]string
}

// httpapiSeatClaim aliases the handler's claim type. The test lives in the
// same package, so it is SeatClaim; the alias keeps the struct literal
// lines short.
type httpapiSeatClaim = httpapi.SeatClaim

func (g *mapGate) mint(t host.TableID, seat int) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	tok := fmt.Sprintf("game-token-%d", g.next)
	claim := httpapiSeatClaim{Table: t, Seat: state.PlayerID(seat)}
	g.tokens[tok] = claim
	g.claims[claim] = tok
	return tok, nil
}

func (g *mapGate) token(t host.TableID, seat state.PlayerID) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.claims[httpapiSeatClaim{Table: t, Seat: seat}]
}

func (g *mapGate) resolve(r *http.Request) (httpapiSeatClaim, bool) {
	tok := eventToken(r)
	g.mu.Lock()
	defer g.mu.Unlock()
	claim, ok := g.tokens[tok]
	return claim, ok
}

// callJSON performs one JSON API call with an optional bearer token,
// decoding a 2xx body into out (when non-nil) and returning the status with
// any error body. It is the e2e's http client: every request is one
// untrusted call through the same wire the browser uses.
func callJSON(t *testing.T, method, url, token string, body, out any) (int, protocol.ErrorBody) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("%s %s: non-JSON error body %q", method, url, raw)
		}
		return resp.StatusCode, e
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s %s: %v\n%s", method, url, err, raw)
		}
	}
	return resp.StatusCode, protocol.ErrorBody{}
}

// bodyJSON serialises a decoded response the way the server wrote it, for
// the seat-scoped leak assertions.
func bodyJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// eventToken reads a request's bearer token or ?token= query exactly like
// gorge's httpapi (server/host/httpapi/events.go eventToken).
func eventToken(r *http.Request) string {
	const prefix = "Bearer "
	if v := r.Header.Get("Authorization"); len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return v[len(prefix):]
	}
	return r.URL.Query().Get("token")
}

// sseFrame is one parsed server-sent event.
type sseFrame struct {
	id    string
	event string
	frame protocol.Frame
}

// openStream opens the SSE stream at url+"/api/stream".
func openStream(t *testing.T, ctx context.Context, url, lastID string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url+"/api/stream", nil)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return resp
}

// collectEventFrames reads the SSE body into out until the stream closes,
// then closes done. It is the goroutine half of readSSE: no t calls, a
// buffered channel, frames dropped only if the channel is somehow full.
func collectEventFrames(body io.Reader, out chan<- sseFrame, done chan<- struct{}) {
	defer close(done)
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var cur sseFrame
	var data bytes.Buffer
	flush := func() {
		if data.Len() == 0 {
			return
		}
		var f protocol.Frame
		if json.Unmarshal(data.Bytes(), &f) == nil {
			cur.frame = f
			select {
			case out <- cur:
			default:
			}
		}
		cur, data = sseFrame{}, bytes.Buffer{}
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "id: "):
			cur.id = line[4:]
		case strings.HasPrefix(line, "event: "):
			cur.event = line[7:]
		case strings.HasPrefix(line, "data: "):
			data.WriteString(line[6:])
		}
	}
	flush()
}

// capturedPack is one seat-scoped draft_pack body the human seat read, in
// capture order: the pack coordinates the body itself names (Round is the
// pack's round; PickNo is seat 0's own pick count within it), the card ids
// on offer, the pool the body carried, and the card seat 0 picked from it.
type capturedPack struct {
	round  int
	pickNo int
	body   string
	ids    []string
	pool   []string
	taken  string
}

// containsID reports whether the card id list carries id.
func containsID(ids []string, id string) bool {
	for _, have := range ids {
		if have == id {
			return true
		}
	}
	return false
}

// logEntry is the one log-entry shape the e2e reads back: kind, acting seat
// and the pick fields.
type logEntry struct {
	Kind   string `json:"kind"`
	Seat   int    `json:"seat"`
	Round  int    `json:"round"`
	PickNo int    `json:"pick_no"`
	ID     string `json:"id"`
}

// TestDraftEventEndToEndOverHTTP drives one 1-human + 3-bot FDN event
// through the served HTTP surface end to end: create, subscribe, join,
// draft, build, ready, three rounds, COMPLETE — and asserts along the way
// that seat 0's own responses never carry another seat's drafted cards.
func TestDraftEventEndToEndOverHTTP(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	store, err := eventhost.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	relay := &eventhost.Relay{}
	r, err := host.New(host.Options{
		Dir: t.TempDir(), LoadDeck: store.DeckLoader(reg, nil), Tokens: reg.Tokens,
		NameUniverse: reg.Universe(), ThinkTimeout: time.Millisecond,
		BotUnboundedDecisions: true, MaxDecisionsPerTurn: host.DefaultMaxDecisionsPerTurn,
		OnMatchEnd: relay.OnMatchEnd,
		Sleep:      func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	gate := &mapGate{tokens: map[string]httpapiSeatClaim{}, claims: map[httpapiSeatClaim]string{}}
	evh, err := eventhost.New(eventhost.Options{
		Registry: r, Store: store, Cards: reg,
		DefaultPolicy: "bot", Mulligans: 1, MintSeat: gate.mint,
	})
	if err != nil {
		r.Close()
		t.Fatal(err)
	}
	relay.Set(evh.OnMatchEnd)
	srv := httptest.NewServer(httpapi.NewHandler(r, httpapi.Options{Events: evh, Seat: gate.resolve}))
	t.Cleanup(func() { evh.Close(); r.Close(); srv.Close() })

	// The stream: open first, take the hello, then keep a collector reading
	// every frame the event ever publishes to seat 0.
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	frames := make(chan sseFrame, 4096)
	streamDone := make(chan struct{})
	go collectEventFrames(resp.Body, frames, streamDone)
	t.Cleanup(func() {
		cancel()
		<-streamDone
	})
	hello := <-frames
	if hello.event != "hello" {
		t.Fatalf("first stream frame %q, want hello", hello.event)
	}
	var h protocol.Hello
	_ = hello.frame.Decode(&h)
	if h.Session == "" {
		t.Fatal("hello carries no session id")
	}

	// Create the event: seat 0 human, seats 1-3 bots on three policies.
	req := protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
		{Kind: "human"}, {Kind: "bot", Policy: "bot"},
		{Kind: "bot", Policy: "lethal-pressure"}, {Kind: "bot", Policy: "forge-heuristic"},
	}}
	var created protocol.CreateEventResponse
	if code, e := callJSON(t, "POST", srv.URL+"/api/events", "", req, &created); code != 200 {
		t.Fatalf("create event: %d %s", code, e.Code)
	}
	if created.Event != "e1" || len(created.Invites) != 1 || created.Invites[0].Seat != 0 {
		t.Fatalf("create reply %+v: want e1 and one human invite", created)
	}
	if created.Invites[0].Token == "" || !strings.Contains(created.Invites[0].Join, "token=") {
		t.Fatalf("human invite %+v carries no token/join link", created.Invites[0])
	}
	tok0 := created.Invites[0].Token
	evURL := srv.URL + "/api/events/e1"

	// Subscribe the stream to the event as seat 0.
	if code, e := callJSON(t, "POST", evURL+"/subscribe", tok0, protocol.EventSubscribe{Session: h.Session}, nil); code != 204 {
		t.Fatalf("subscribe: %d %s", code, e.Code)
	}

	// Join as Ann: the draft opens the moment the event's only human has a
	// name.
	var joined protocol.EventState
	if code, e := callJSON(t, "POST", evURL+"/join", tok0, protocol.JoinEventRequest{Name: "Ann"}, &joined); code != 200 {
		t.Fatalf("join: %d %s", code, e.Code)
	}
	if joined.Phase != "DRAFT" {
		t.Fatalf("phase after join %q, want DRAFT", joined.Phase)
	}

	// The draft: pick the first card of every pack the queue head offers,
	// re-sending the first pick once to prove a stale answer is a 409.
	var packs []capturedPack
	staleChecked := false
	deadline := time.Now().Add(50 * time.Second)
	for {
		var st protocol.EventState
		code, e := callJSON(t, "GET", evURL, tok0, nil, &st)
		if code != 200 {
			t.Fatalf("state: %d %s", code, e.Code)
		}
		if st.Phase == "BUILD" {
			break
		}
		if st.Phase != "DRAFT" {
			t.Fatalf("phase %q mid-draft, want DRAFT or BUILD", st.Phase)
		}
		if time.Now().After(deadline) {
			t.Fatalf("draft not over in 50s (phase %s)", st.Phase)
		}
		var pack protocol.DraftPack
		code, e = callJSON(t, "GET", evURL+"/pack", tok0, nil, &pack)
		if code != 200 {
			t.Fatalf("pack: %d %s", code, e.Code)
		}
		if pack.Waiting || len(pack.Pack) == 0 {
			time.Sleep(eventsPollEvery)
			continue
		}
		ids := make([]string, 0, len(pack.Pack))
		for _, c := range pack.Pack {
			ids = append(ids, c.ID)
		}
		pool := make([]string, 0, len(pack.Pool))
		for _, c := range pack.Pool {
			pool = append(pool, c.ID)
		}
		taken := pack.Pack[0].ID
		packs = append(packs, capturedPack{round: pack.Round, pickNo: pack.PickNo,
			body: bodyJSON(t, pack), ids: ids, pool: pool, taken: taken})
		pick := protocol.PickRequest{Round: pack.Round, PickNo: pack.PickNo, ID: taken}
		var next protocol.DraftPack
		code, e = callJSON(t, "POST", evURL+"/pick", tok0, pick, &next)
		if code != 200 {
			t.Fatalf("pick round %d no %d: %d %s", pack.Round, pack.PickNo, code, e.Code)
		}
		if !staleChecked {
			staleChecked = true
			code, e = callJSON(t, "POST", evURL+"/pick", tok0, pick, nil)
			if code != 409 || e.Code != "stale_pick" {
				t.Fatalf("re-sent pick: %d %s, want 409 stale_pick", code, e.Code)
			}
		}
	}

	// The spectator log mid-draft: every Picked's card id is redacted.
	var midLog protocol.EventLog
	if code, e := callJSON(t, "GET", evURL+"/log", "", nil, &midLog); code != 200 {
		t.Fatalf("mid-draft log: %d %s", code, e.Code)
	}
	if midLog.Complete {
		t.Fatal("mid-draft log reports complete")
	}
	for _, raw := range midLog.Entries {
		var en logEntry
		if err := json.Unmarshal(raw, &en); err != nil {
			t.Fatalf("mid-draft log entry %s: %v", raw, err)
		}
		if en.Kind == "Picked" && en.ID != "" {
			t.Fatalf("mid-draft spectator log leaks seat %d's pick %q", en.Seat, en.ID)
		}
	}

	// Build: the suggestion validates, saves as rev 1, and readying rev 1
	// opens round 1 (the bots save and ready themselves during advance).
	var pool protocol.EventPool
	if code, e := callJSON(t, "GET", evURL+"/pool", tok0, nil, &pool); code != 200 {
		t.Fatalf("pool: %d %s", code, e.Code)
	}
	if len(pool.Pool) == 0 {
		t.Fatal("empty pool at BUILD: the draft's picks never reached the seat")
	}
	poolIDs := make([]string, 0, len(pool.Pool))
	for _, c := range pool.Pool {
		poolIDs = append(poolIDs, c.ID)
	}
	var check protocol.DeckCheck
	code, e := callJSON(t, "POST", evURL+"/deck/check", tok0, pool.Suggestion, &check)
	if code != 200 || !check.Valid {
		t.Fatalf("deck/check of the suggestion: %d %s valid=%v err=%q", code, e.Code, check.Valid, check.Error)
	}
	var saved protocol.DeckCheck
	code, e = callJSON(t, "POST", evURL+"/deck", tok0, pool.Suggestion, &saved)
	if code != 200 || saved.Rev != 1 {
		t.Fatalf("deck save: %d %s rev=%d", code, e.Code, saved.Rev)
	}
	var readied protocol.EventState
	code, e = callJSON(t, "POST", evURL+"/ready", tok0, protocol.ReadyRequest{Rev: 1}, &readied)
	if code != 200 {
		t.Fatalf("ready: %d %s", code, e.Code)
	}

	// Round 1: the seat's own state names the (0,3) series' join link, the
	// public state carries none, and the link's game token resolves the
	// seat-scoped match view.
	var series protocol.EventSeries
	deadline = time.Now().Add(10 * time.Second)
	for {
		var st protocol.EventState
		code, e := callJSON(t, "GET", evURL, tok0, nil, &st)
		if code != 200 {
			t.Fatalf("state: %d %s", code, e.Code)
		}
		found := false
		for _, s := range st.Series {
			if s.Round == 1 && s.A == 0 && s.B == 3 {
				series = s
				found = true
			}
		}
		if found && series.State == "live" && series.Join != "" && series.Table != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("round-1 (0,3) series never went live with a join link: %+v", series)
		}
		time.Sleep(eventsPollEvery)
	}
	u, err := url.Parse(series.Join)
	if err != nil {
		t.Fatalf("join link %q: %v", series.Join, err)
	}
	gameTok := u.Query().Get("token")
	if gameTok == "" || u.Query().Get("seat") != "0" {
		t.Fatalf("join link %q: want seat=0 and a token", series.Join)
	}
	if st := publicState(t, evURL); strings.Contains(st, `"join":"`) {
		t.Fatalf("public state carries a join link: %s", st)
	}
	deadline = time.Now().Add(10 * time.Second)
	for {
		code, e := callJSON(t, "GET", srv.URL+"/api/tables/"+series.Table+"/matches/1/view?seat=0&token="+gameTok, "", nil, nil)
		if code == 200 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("seat-scoped game view never answered 200 (last %d %s)", code, e.Code)
		}
		time.Sleep(eventsPollEvery)
	}

	// The rounds: the event completes with a sane standings table and a
	// fully unredacted public log.
	deadline = time.Now().Add(50 * time.Second)
	var final protocol.EventState
	for {
		code, e := callJSON(t, "GET", evURL, tok0, nil, &final)
		if code != 200 {
			t.Fatalf("state: %d %s", code, e.Code)
		}
		if final.Phase == "COMPLETE" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("event not COMPLETE in 50s (phase %s)", final.Phase)
		}
		time.Sleep(eventsPollEvery)
	}
	if final.You == nil || *final.You != 0 {
		t.Fatalf("token'd state You=%v, want 0", final.You)
	}
	if len(final.Standings) != 4 {
		t.Fatalf("standings rows %d, want 4", len(final.Standings))
	}
	points := 0
	for _, s := range final.Standings {
		points += s.Points
	}
	if points < 12 || points > 18 {
		t.Fatalf("standings points sum %d, want 12..18: %+v", points, final.Standings)
	}
	var endLog protocol.EventLog
	if code, e := callJSON(t, "GET", evURL+"/log", "", nil, &endLog); code != 200 {
		t.Fatalf("final log: %d %s", code, e.Code)
	}
	if !endLog.Complete {
		t.Fatal("final log reports incomplete")
	}
	var entries []logEntry
	for _, raw := range endLog.Entries {
		var en logEntry
		if err := json.Unmarshal(raw, &en); err != nil {
			t.Fatalf("final log entry %s: %v", raw, err)
		}
		entries = append(entries, en)
	}
	seriesEnded := 0
	pickedBy := map[int]map[string]bool{0: {}, 1: {}, 2: {}, 3: {}}
	pickAt0 := map[string]logEntry{} // seat 0's pick of each id, with coordinates
	for _, en := range entries {
		switch en.Kind {
		case "SeriesEnded":
			seriesEnded++
		case "Picked":
			if en.ID == "" {
				t.Fatalf("COMPLETE log still redacts seat %d's pick", en.Seat)
			}
			pickedBy[en.Seat][en.ID] = true
			if en.Seat == 0 {
				pickAt0[en.ID] = en
			}
		}
	}
	if seriesEnded != 6 {
		t.Fatalf("SeriesEnded entries %d, want 6", seriesEnded)
	}

	// Seat privacy, asserted over every seat-scoped body seat 0 read:
	// - the pool body held exactly seat 0's own picks and no other seat's
	//   card id anywhere in its serialisation;
	// - each pack body's pool equalled exactly the cards seat 0 took from
	//   the packs captured before it;
	// - each pack body's on-offer ids contained seat 0's taken card, no
	//   other card seat 0 ever owned *before this view* — a pack only
	//   shrinks as it travels, so a card still on offer when seat 0 saw it
	//   was picked, if at all, at a strictly later pick number of the same
	//   round (PickNo is per-seat; the comparison pins "after seat 0's view
	//   of this pack", which is what the redaction needs). Pack visits
	//   repeat, so the same card may be offered by several captures.
	own := pickedBy[0]
	if len(own) == 0 {
		t.Fatal("no seat-0 picks in the log: precondition missing")
	}
	if len(poolIDs) != len(own) {
		t.Fatalf("pool holds %d cards, seat 0 picked %d", len(poolIDs), len(own))
	}
	poolBody := bodyJSON(t, pool)
	for seat := 1; seat <= 3; seat++ {
		for id := range pickedBy[seat] {
			if strings.Contains(poolBody, `"id":"`+id+`"`) {
				t.Fatalf("seat 0's pool body leaks seat %d's card %q", seat, id)
			}
		}
	}
	for _, id := range poolIDs {
		if !own[id] {
			t.Fatalf("seat 0's pool contains %q, which seat 0 never picked", id)
		}
	}
	if len(packs) == 0 {
		t.Fatal("no draft pack was ever captured: precondition missing")
	}
	for i, cp := range packs {
		if len(cp.ids) == 0 {
			t.Fatalf("captured pack %d is empty: precondition missing", i)
		}
		prior := map[string]bool{}
		for _, prev := range packs[:i] {
			prior[prev.taken] = true
		}
		if len(cp.pool) != len(prior) {
			t.Fatalf("pack r%d p%d's pool holds %d cards, want the %d taken from the packs before it",
				cp.round, cp.pickNo, len(cp.pool), len(prior))
		}
		for _, id := range cp.pool {
			if !prior[id] {
				t.Fatalf("pack r%d p%d's pool contains %q, not taken from any earlier pack", cp.round, cp.pickNo, id)
			}
		}
		if !containsID(cp.ids, cp.taken) {
			t.Fatalf("pack r%d p%d does not offer %q, the card seat 0 picked from it", cp.round, cp.pickNo, cp.taken)
		}
		for _, id := range cp.ids {
			if en := pickAt0[id]; own[id] && (en.Round != cp.round || en.PickNo < cp.pickNo) {
				t.Fatalf("pack r%d p%d offers %q, which seat 0 picked at round %d pick %d, before this view",
					cp.round, cp.pickNo, id, en.Round, en.PickNo)
			}
			for _, en := range entries {
				if en.Kind != "Picked" || en.ID != id || en.Seat == 0 {
					continue
				}
				if en.Round != cp.round || en.PickNo <= cp.pickNo {
					t.Fatalf("pack r%d p%d leaks %q: seat %d picked it at round %d pick %d, not after seat 0's view",
						cp.round, cp.pickNo, id, en.Seat, en.Round, en.PickNo)
				}
			}
		}
	}

	// The stream saw every event frame kind the draft and the rounds publish.
	cancel()
	<-streamDone
	seen := map[string]int{}
drain:
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				break drain
			}
			seen[f.event]++
		default:
			break drain
		}
	}
	for _, want := range []string{"event_state", "draft_pack", "pod_progress", "series_update"} {
		if seen[want] == 0 {
			t.Fatalf("stream never delivered a %q frame; saw %v", want, seen)
		}
	}
}

// publicState fetches the event state with no token and returns its raw
// body, for the join-link absence assertion.
func publicState(t *testing.T, evURL string) string {
	t.Helper()
	req, err := http.NewRequest("GET", evURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("public state: %d %s", resp.StatusCode, string(raw))
	}
	return string(raw)
}
