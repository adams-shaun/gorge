package sbv2http

// Shared corpus driver for TestSBV2ParityLegacyDecks and
// TestSBV2NativeOnlyCensus. Both run the same 20 fixed seed cases: a native
// table whose two seats are the production default bots, and a /api/sbv2
// table at the same seed whose seat 0 is a scripted HTTP client that answers
// every decision with the production bot's own pick. The native table is the
// reference; the wire table must reach the same event-log chain head.
//
// The two tests share one run so the smoke-sized corpus (20 seeds, 2 workers)
// is not paid twice under the package's 3-minute test budget. A run is
// memoised for the life of the test binary; a failed run returns its errors
// to every caller that asks, so neither test can pass vacuously because the
// other ran first.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	hostprotocol "github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// paritySeeds is the smoke-sized corpus: 20 fixed table seeds over the same
// 2-seat Legacy deck-pair scheme server/host/manabrewhttp's parity test uses
// (seats*1000+g). Two workers cover it well under the 3-minute budget.
var paritySeeds = func() []uint64 {
	out := make([]uint64, 20)
	for i := range out {
		out[i] = 2000 + uint64(i)
	}
	return out
}()

// parityWorkers is the brief's worker count.
const parityWorkers = 2

// parityDeadline bounds one seed's whole run (both tables and the client).
// The games finish in well under a second; the deadline exists so a stalled
// client fails the seed with its error rather than the package timeout.
const parityDeadline = 90 * time.Second

type seedOutcome struct {
	seed         uint64
	nativeHead   string
	wireHead     string
	nativeEvents int
	wireEvents   int
	// census counts the native-only decisions the scripted client was served,
	// keyed by native decision kind.
	census map[string]int
	// picks counts the mapped (non-native_only) decisions the client saw, and
	// sends how many of them it answered over /send. misses is a strict subset
	// of picks: the production bot's native pick had no v2 candidate, so the
	// client answered it over the native routes. All three are ratcheted by
	// TestSBV2ParityLegacyDecks.
	picks  int
	sends  int
	misses int
	errs   []string
}

type parityCorpusResult struct {
	outcomes []seedOutcome
	errs     []string
}

var parityCorpusCache struct {
	mu   sync.Mutex
	done bool
	res  parityCorpusResult
}

// parityCorpus runs (or returns the memoised result of) the 20-seed corpus.
// It must be called from a test goroutine: testutil.CorpusRegistry may skip
// or fail the test, and a skip leaves the cache empty for the next caller.
func parityCorpus(t *testing.T) parityCorpusResult {
	t.Helper()
	parityCorpusCache.mu.Lock()
	defer parityCorpusCache.mu.Unlock()
	if parityCorpusCache.done {
		return parityCorpusCache.res
	}
	reg := testutil.CorpusRegistry(t)

	jobs := make(chan uint64)
	results := make(chan seedOutcome, len(paritySeeds))
	var wg sync.WaitGroup
	for w := 0; w < parityWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range jobs {
				results <- runParitySeed(reg, seed)
			}
		}()
	}
	for _, seed := range paritySeeds {
		jobs <- seed
	}
	close(jobs)
	wg.Wait()
	close(results)

	res := parityCorpusResult{outcomes: make([]seedOutcome, 0, len(paritySeeds))}
	for out := range results {
		res.outcomes = append(res.outcomes, out)
	}
	sort.Slice(res.outcomes, func(i, j int) bool { return res.outcomes[i].seed < res.outcomes[j].seed })
	if len(res.outcomes) != len(paritySeeds) {
		res.errs = append(res.errs, fmt.Sprintf("corpus produced %d seed outcomes, want %d", len(res.outcomes), len(paritySeeds)))
	}
	parityCorpusCache.res = res
	parityCorpusCache.done = true
	return res
}

// parityTableConfig seats one seed's two Legacy decks. The table seed is the
// only input; both tables of a case get exactly this configuration apart from
// ID/Name/Humans, so match seed, decks and bot seeds agree.
func parityTableConfig(id host.TableID, seed uint64) host.TableConfig {
	names := testutil.LegacyDeckNames()
	return host.TableConfig{
		ID: id, Name: string(id), Seats: 2,
		Decks: []string{names[int(seed)%len(names)], names[(int(seed)+1)%len(names)]},
		Seed:  seed, Mulligans: 0, Spectator: view.Omniscient,
		AutoMana: true, BotAutoPayMana: true,
		BotPolicy: host.BotPolicy,
	}
}

// parityOptions is the live host both tables share. BotUnboundedDecisions
// disarms the hosted wall-clock budget, so the native reference and the
// scripted client's production-bot picks are pure functions of their seeds
// (a deadline bail-out would otherwise make the comparison load-dependent).
func parityOptions(reg *cards.Registry, hub *Hub) host.Options {
	return host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			cs, err := testutil.LoadRepoDeck(reg, name)
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Tokens:                reg.Tokens,
		NameUniverse:          reg.Universe(),
		Sleep:                 func(time.Duration, <-chan struct{}) {},
		TapHuman:              hub.Tap,
		BotUnboundedDecisions: true,
	}
}

// productionBot is the default hosted seat the native table builds for seat
// seatIndex of match 1: bots.New with the match seed, auto-payment, no search
// parallelism, no card deps and no per-decision deadline (the table's
// BotUnboundedDecisions posture). Building it here, rather than through
// host.NewBotPolicySeatWithAutoPayMana, keeps the client's bot byte-for-byte
// the native table's bot.
func productionBot(tableSeed uint64, seatIndex int) (seat.Seat, error) {
	return bots.New(host.BotPolicy, bots.Options{
		Seed:        host.MatchSeed(tableSeed, 1) ^ uint64(seatIndex+1),
		AutoPayMana: true,
	})
}

// runParitySeed hosts one seed's native and wire tables, drives the wire
// client to game over, and compares the chain heads.
func runParitySeed(reg *cards.Registry, seed uint64) seedOutcome {
	out := seedOutcome{seed: seed, census: map[string]int{}}
	fail := func(format string, args ...any) seedOutcome {
		out.errs = append(out.errs, fmt.Sprintf(format, args...))
		return out
	}

	hub := NewHub()
	r, err := host.New(parityOptions(reg, hub))
	if err != nil {
		return fail("host.New: %v", err)
	}
	defer r.Close()

	native := host.TableID(fmt.Sprintf("parity-native-%d", seed))
	wire := host.TableID(fmt.Sprintf("parity-wire-%d", seed))
	cfgNative := parityTableConfig(native, seed)
	cfgWire := cfgNative
	cfgWire.ID = wire
	cfgWire.Name = string(wire)
	cfgWire.Humans = []int{0}
	if err := r.AddTable(cfgNative); err != nil {
		return fail("AddTable(native): %v", err)
	}
	if err := r.AddTable(cfgWire); err != nil {
		return fail("AddTable(wire): %v", err)
	}

	const token = "parity-s0"
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		if strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ") != token {
			return SeatClaim{}, false
		}
		return SeatClaim{Table: wire, Seat: 0}, true
	}})))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if err := r.Start(native); err != nil {
		return fail("Start(native): %v", err)
	}
	if err := r.Start(wire); err != nil {
		return fail("Start(wire): %v", err)
	}

	bot, err := productionBot(seed, 0)
	if err != nil {
		return fail("production bot for seat 0: %v", err)
	}
	clientDone := make(chan parityClientResult, 1)
	go func() {
		clientDone <- playParityClient(r, hub, srv.URL, wire, token, bot)
	}()

	headNative, eventsNative, err := waitParityMatch(r, native)
	if err != nil {
		// Stop the wire client before returning so it cannot outlive this
		// seed's registry; the client returns once the table is closed.
		_ = r.Close()
		select {
		case <-clientDone:
		case <-time.After(5 * time.Second):
		}
		return fail("native table: %v", err)
	}
	out.nativeHead, out.nativeEvents = headNative, eventsNative

	client := <-clientDone
	out.misses, out.sends, out.picks, out.census = client.misses, client.sends, client.picks, client.census
	if client.err != nil {
		return fail("wire client: %v", client.err)
	}
	headWire, eventsWire, err := waitParityMatch(r, wire)
	if err != nil {
		return fail("wire table: %v", err)
	}
	out.wireHead, out.wireEvents = headWire, eventsWire
	if headNative != headWire {
		return fail("chain heads differ: native %s (%d events), wire %s (%d events); first event divergence: %s",
			headNative, eventsNative, headWire, eventsWire, parityFirstEventDiff(r, native, wire))
	}
	return out
}

type parityClientResult struct {
	picks  int
	sends  int
	misses int
	census map[string]int
	err    error
}

// playParityClient is the scripted /api/sbv2 client: it polls the state route
// for seat 0, answers every mapped decision with the candidate the production
// bot's native intent maps onto, answers a native_only decision through the
// native routes, and counts those native_only decisions by kind. When the
// bot's native answer is not among the v2 candidates it gives v2 up for that
// decision and answers it natively, the same fallback humanseat's scripted
// answerer documents.
func playParityClient(r *host.Registry, hub *Hub, base string, table host.TableID, token string, bot seat.Seat) parityClientResult {
	p := &picker{bot: bot}
	census := map[string]int{}
	ss := hub.seat(seatKey{table: table, match: 1, seat: 0})
	statePath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/state", table)
	sendPath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/send", table)
	deadline := time.Now().Add(parityDeadline)
	handled := map[string]bool{}
	seenNative := map[string]bool{}
	sends, picks := 0, 0
	result := func(err error) parityClientResult {
		return parityClientResult{picks: picks, sends: sends, misses: p.misses, census: census, err: err}
	}

	for {
		if done, _, err := parityMatchState(r, table); err != nil {
			return result(err)
		} else if done {
			return result(nil)
		}
		if time.Now().After(deadline) {
			return result(fmt.Errorf("client deadline expired while polling %s", statePath))
		}

		status, body, err := parityGet(base+statePath, token)
		if err != nil {
			return result(fmt.Errorf("GET state: %w", err))
		}
		if status != http.StatusOK {
			return result(fmt.Errorf("GET state: status %d body %s", status, body))
		}
		var doc stateBody
		if err := json.Unmarshal(body, &doc); err != nil {
			return result(fmt.Errorf("state JSON: %v (%s)", err, body))
		}
		if doc.Request == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		id := doc.Request.RequestID
		if handled[id] {
			time.Sleep(time.Millisecond)
			continue
		}
		// Confirm the state route and the hub agree: a state body can be in
		// flight while the decision is superseded or answered.
		req, _ := ss.snapshot()
		if req == nil || fmt.Sprint(req.id) != id {
			time.Sleep(time.Millisecond)
			continue
		}

		if isNativeOnly(doc.Request.Decision) {
			if !seenNative[id] {
				seenNative[id] = true
				census[nativeOnlyKind(doc.Request)]++
			}
			if err := answerParityNative(r, table, bot); err != nil {
				// Pending can lag the synthetic frame by a scheduling tick;
				// a superseding v2 request is handled on the next pass.
				time.Sleep(time.Millisecond)
				continue
			}
			handled[id] = true
			continue
		}

		picks++
		cand, miss := p.pick(req.ask)
		if miss != "" {
			// picker.pick already counted this miss; counting it here too
			// would double the ratcheted baseline. The production bot's
			// native answer is not among the v2 candidates (an engine
			// default or an option the wire folds or hides). Give v2 up for
			// this decision and answer it over the native routes, exactly as
			// humanseat's scripted answerer does when agent.Pick misses.
			if err := r.SubmitIntent(table, 1, 0, p.intent); err != nil {
				return result(fmt.Errorf("native fallback for seq %d (%s): %w", p.planSeq, req.ask.Pose.Native.Kind, err))
			}
			handled[id] = true
			continue
		}
		sendBody, err := json.Marshal(map[string]any{"request_id": id, "candidate_id": cand})
		if err != nil {
			return result(fmt.Errorf("marshal send: %w", err))
		}
		status, respBody, err := parityPost(base+sendPath, token, sendBody)
		if err != nil {
			return result(fmt.Errorf("POST send: %w", err))
		}
		if status == http.StatusNoContent {
			sends++
			handled[id] = true
			continue
		}
		if status == http.StatusConflict {
			// Lost the race to a superseding request: re-read state.
			time.Sleep(time.Millisecond)
			continue
		}
		return result(fmt.Errorf("POST send request %s candidate %d: status %d body %s", id, cand, status, respBody))
	}
}

// answerParityNative answers the pending native decision with the production
// bot, exactly as a client following a native_only frame does.
func answerParityNative(r *host.Registry, table host.TableID, bot seat.Seat) error {
	d, err := r.Pending(table, 1, 0)
	if err != nil {
		return err
	}
	seq, err := sbv2HeadSeq(r, table, 1)
	if err != nil {
		return err
	}
	v, err := r.ViewAtSeat(table, 1, seq, 0)
	if err != nil {
		return err
	}
	v.Decision = d
	in, err := bot.Decide(context.Background(), v, *d)
	if err != nil {
		return err
	}
	return r.SubmitIntent(table, 1, 0, in)
}

// nativeOnlyKind reads the native decision kind out of the render extension
// that accompanies x_gorge_human_v1.native_only.
func nativeOnlyKind(req *chooseRequest) string {
	raw, ok := req.Decision.Extensions["x_gorge_view_v1"]
	if !ok {
		return "unknown"
	}
	var payload xview.Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "unknown"
	}
	return string(payload.Decision.Kind)
}

// parityMatchState reports whether match 1 of the table is terminal.
func parityMatchState(r *host.Registry, table host.TableID) (bool, string, error) {
	ms, err := r.Matches(table)
	if err != nil {
		return false, "", err
	}
	for _, m := range ms {
		if m.Match != 1 {
			continue
		}
		return m.State != hostprotocol.MatchLive, m.State, nil
	}
	return false, "", nil
}

// waitParityMatch polls until match 1 finishes and returns its head.
func waitParityMatch(r *host.Registry, table host.TableID) (string, int, error) {
	deadline := time.Now().Add(parityDeadline)
	for {
		ms, err := r.Matches(table)
		if err == nil {
			for _, m := range ms {
				if m.Match != 1 {
					continue
				}
				switch m.State {
				case hostprotocol.MatchFinished:
					if m.Head == "" {
						return "", 0, fmt.Errorf("match 1 finished with an empty chain head")
					}
					return m.Head, m.Events, nil
				case hostprotocol.MatchCrashed, hostprotocol.MatchAborted:
					return "", 0, fmt.Errorf("match 1 ended %s: %s", m.State, m.Head)
				}
			}
		}
		if time.Now().After(deadline) {
			return "", 0, fmt.Errorf("match 1 did not finish within %v", parityDeadline)
		}
		time.Sleep(time.Millisecond)
	}
}

// parityFirstEventDiff names the first event that differs between the two
// finished logs, for a failure message that points at the behavior rather
// than just the hash.
func parityFirstEventDiff(r *host.Registry, a, b host.TableID) string {
	ea, err := r.Events(a, 1, 0)
	if err != nil {
		return fmt.Sprintf("reading native events: %v", err)
	}
	eb, err := r.Events(b, 1, 0)
	if err != nil {
		return fmt.Sprintf("reading wire events: %v", err)
	}
	for i := 0; i < len(ea) && i < len(eb); i++ {
		if reflect.DeepEqual(ea[i], eb[i]) {
			continue
		}
		return fmt.Sprintf("event %d: native %+v, wire %+v", i, ea[i].Event, eb[i].Event)
	}
	return fmt.Sprintf("logs share a %d-event prefix (native %d, wire %d)", min(len(ea), len(eb)), len(ea), len(eb))
}

func parityGet(url, token string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

func parityPost(url, token string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}
