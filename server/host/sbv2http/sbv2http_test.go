package sbv2http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// recorder keeps every intent the v2 seat committed, by native decision seq.
// The native reference table replays exactly this stream, so the two heads
// must agree whatever the mapping chose to fold.
type recorder struct {
	mu sync.Mutex
	in map[uint64]decision.Intent
}

func (r *recorder) add(in decision.Intent) {
	r.mu.Lock()
	if r.in == nil {
		r.in = map[uint64]decision.Intent{}
	}
	r.in[in.Seq] = in
	r.mu.Unlock()
}

func (r *recorder) get(seq uint64) (decision.Intent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.in[seq]
	return in, ok
}

// recTap wraps the hub's V2HumanSeat so the test sees the intents it commits.
type recTap struct {
	host.HumanTap
	rec *recorder
}

func (r recTap) Parked(ctx context.Context, v view.View, d decision.Decision, answer func(decision.Intent) error, gone <-chan struct{}) {
	r.HumanTap.Parked(ctx, v, d, func(in decision.Intent) error {
		err := answer(in)
		if err == nil {
			r.rec.add(in)
		}
		return err
	}, gone)
}

// picker answers each v2 substep the way the production bot answers the native
// decision: it plans on the native decision/view the codec carries and picks
// the candidate that carries the plan out.
type picker struct {
	bot     seat.Seat
	plan    *agent.Plan
	planSeq uint64
	intent  decision.Intent
	misses  int
}

func (p *picker) pick(ask humanseat.Ask) (int, string) {
	ctx := context.Background()
	nd := *ask.Pose.Native
	askBot := func(d decision.Decision) decision.Intent {
		v := ask.View
		v.Decision = &d
		in, err := p.bot.Decide(ctx, v, d)
		if err != nil {
			panic(err)
		}
		return in
	}
	if p.plan == nil || p.planSeq != nd.Seq {
		p.intent = askBot(nd)
		p.plan, p.planSeq = agent.NewPlan(nd.Seq, p.intent), nd.Seq
	}
	ops := make([]mapping.NativeOp, len(ask.Pose.Candidates))
	sems := make([]map[string]any, len(ask.Pose.Candidates))
	for i, c := range ask.Pose.Candidates {
		ops[i] = c.Op
		m := map[string]any{"kind": c.Sem.Kind}
		for k, v := range c.Sem.Fields {
			m[k] = v
		}
		sems[i] = m
	}
	follow := map[string]decision.Decision{}
	for k, f := range ask.Pose.Followups {
		follow[k] = *f
	}
	i, miss := agent.Pick(xview.Payload{Decision: nd, Ops: ops, Followups: follow}, sems, p.plan, askBot)
	if miss != "" {
		p.misses++
	}
	return i, miss
}

// sbv2RepoOptions is the live host the v2 routes drive. TapHuman is the same
// hook server/gorged installs; the recorder only observes the intents.
func sbv2RepoOptions(t *testing.T, reg *cards.Registry, hub *Hub, rec *recorder) host.Options {
	t.Helper()
	return host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			cs, err := testutil.LoadRepoDeck(reg, name)
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
		Sleep:        func(time.Duration, <-chan struct{}) {},
		TapHuman: func(table host.TableID, match int, autoPayMana bool, mc *bots.MatchContext) (host.HumanTap, error) {
			tap, err := hub.Tap(table, match, autoPayMana, mc)
			if err != nil || tap == nil {
				return tap, err
			}
			return recTap{HumanTap: tap, rec: rec}, nil
		},
	}
}

func sbv2TableCfg(id host.TableID, seed uint64) host.TableConfig {
	names := testutil.LegacyDeckNames()
	return host.TableConfig{ID: id, Name: string(id), Seats: 2,
		Decks: []string{names[int(seed)%len(names)], names[(int(seed)+1)%len(names)]},
		Seed:  seed, Mulligans: 0, Spectator: view.Omniscient, AutoMana: true, BotAutoPayMana: true}
}

// TestSBV2PlaysAGame drives one full two-seat game over the v2 HTTP routes: a
// stream delivers each choose request, state must agree, and send answers with
// the candidate the production bot would pick. The same table then plays
// natively with a plain human seat answered from the intents the v2 seat
// committed; the chain heads must be identical, so the codec changed nothing a
// replay can see.
func TestSBV2PlaysAGame(t *testing.T) {
	const seed = 3000
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	rec := &recorder{}
	o := sbv2RepoOptions(t, reg, hub, rec)
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := sbv2TableCfg("v2", seed)
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	byToken := map[string]SeatClaim{"tok": {Table: "v2", Seat: 0}}
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		c, ok := byToken[strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")]
		return c, ok
	}})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if err := r.Start("v2"); err != nil {
		t.Fatal(err)
	}

	p := &picker{bot: referenceBot(t, seed)}
	stateAgreed := false
	nativeSeen := 0
	events := openSBV2Stream(t, srv.URL, "/api/sbv2/v1/tables/v2/matches/1/stream", "tok")

	deadline := time.Now().Add(2 * time.Minute)
	for {
		if done, head := matchDone(t, r, "v2"); done {
			t.Logf("v2 head %s", head)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("v2 game did not finish")
		}
		// A decision the codec left on the native wire is answered natively,
		// as a client would. The codec's own count is the signal; a null
		// stream frame alone can also mean "between decisions", and answering
		// a merely-about-to-park v2 decision would supersede it.
		if n := hub.seat(seatKey{table: "v2", match: 1, seat: 0}).nativeOnlyCount(); n > nativeSeen {
			if d, err := r.Pending("v2", 1, 0); err == nil {
				seq, herr := sbv2HeadSeq(r, "v2", 1)
				if herr != nil {
					t.Fatalf("native-only head: %v", herr)
				}
				v, verr := r.ViewAtSeat("v2", 1, seq, 0)
				if verr != nil {
					t.Fatalf("native-only view at seq %d: %v", d.Seq, verr)
				}
				v.Decision = d
				in, derr := p.bot.Decide(context.Background(), v, *d)
				if derr != nil {
					t.Fatalf("native-only answer at seq %d: %v", d.Seq, derr)
				}
				if err := r.SubmitIntent("v2", 1, 0, in); err != nil {
					t.Fatalf("native-only submit at seq %d: %v", d.Seq, err)
				}
				rec.add(in)
				nativeSeen = n
				continue
			}
		}
		select {
		case st, open := <-events:
			if !open {
				t.Fatal("v2 stream closed before the game finished")
			}
			if st.Request == nil {
				continue
			}
			// A stream frame can race a newer park: the request it named may
			// already be superseded. Answer only what the hub still calls open.
			req, _ := hub.seat(seatKey{table: "v2", match: 1, seat: 0}).snapshot()
			if req == nil || fmt.Sprint(req.id) != st.Request.RequestID {
				continue
			}
			// state must serve the same open request the stream pushed.
			status, body := getSBV2State(t, srv.URL, "/api/sbv2/v1/tables/v2/matches/1/state", "tok")
			if status != http.StatusOK {
				t.Fatalf("state: %d %q", status, body)
			}
			var stateDoc stateBody
			if err := json.Unmarshal(body, &stateDoc); err != nil {
				t.Fatalf("state body %q: %v", body, err)
			}
			if stateDoc.Request != nil {
				if stateDoc.Request.RequestID != st.Request.RequestID {
					t.Fatalf("state request %+v, stream request %+v", stateDoc.Request, st.Request)
				}
				stateAgreed = true
			}
			cand, miss := p.pick(req.ask)
			if miss != "" {
				// Keep the game playable, but record the substep the
				// production bot cannot name; the native reference still
				// replays whatever the codec commits.
				t.Logf("seq %d: production bot pick missed candidates (%s); answering 0", req.ask.Pose.Native.Seq, miss)
				cand = 0
			}
			if status, body := postSBV2Send(t, srv.URL, "/api/sbv2/v1/tables/v2/matches/1/send", "tok", st.Request.RequestID, cand); status != http.StatusNoContent {
				t.Fatalf("send request %s candidate %d: %d %q", st.Request.RequestID, cand, status, body)
			}
		case <-time.After(time.Millisecond):
		}
	}

	native, skipped := playSBV2NativeReference(t, reg, seed, rec)
	if skipped != 0 {
		t.Fatalf("the native reference left %d decisions unanswered", skipped)
	}
	_, v2head := matchDone(t, r, "v2")
	if v2head != native {
		t.Fatalf("heads differ: v2 HTTP seat %s, native human seat %s", v2head, native)
	}
	if !stateAgreed {
		t.Fatal("state never served the request the stream pushed")
	}
	if p.misses > 0 {
		t.Logf("production bot missed %d substeps (fallback candidate 0)", p.misses)
	}
}

// playSBV2NativeReference plays the same table with a plain human seat and the
// recorded intents, the exact comparison TestV2HumanSeatHeadsMatchNative makes.
func playSBV2NativeReference(t *testing.T, reg *cards.Registry, seed uint64, rec *recorder) (string, int) {
	t.Helper()
	o := host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			cs, err := testutil.LoadRepoDeck(reg, name)
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
		Sleep:        func(time.Duration, <-chan struct{}) {},
		ThinkTimeout: 2 * time.Second,
		Seats: func(names []string, mseed uint64) []seat.Seat {
			out := make([]seat.Seat, len(names))
			for i := range out {
				b, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, mseed^uint64(i+1), true)
				if err != nil {
					t.Fatalf("reference bot: %v", err)
				}
				out[i] = b
			}
			out[0] = host.NewHumanSeat()
			return out
		},
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := sbv2TableCfg("native", seed)
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("native"); err != nil {
		t.Fatal(err)
	}
	skipped := map[uint64]bool{}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if done, head := matchDone(t, r, "native"); done {
			return head, len(skipped)
		}
		if time.Now().After(deadline) {
			t.Fatal("native reference game did not finish")
		}
		d, err := r.Pending("native", 1, 0)
		if err == nil && !skipped[d.Seq] {
			in, ok := rec.get(d.Seq)
			if !ok {
				skipped[d.Seq] = true
				t.Logf("native reference: no intent recorded for seq %d kind %s", d.Seq, d.Kind)
				continue
			}
			if err := r.SubmitIntent("native", 1, 0, in); err != nil {
				t.Fatalf("native replay seq %d: %v", d.Seq, err)
			}
		}
		time.Sleep(time.Millisecond)
	}
}

func sbv2HeadSeq(r *host.Registry, table host.TableID, k int) (uint64, error) {
	ms, err := r.Matches(table)
	if err != nil {
		return 0, err
	}
	for _, m := range ms {
		if m.Match == k {
			if m.Events == 0 {
				return 0, fmt.Errorf("no events")
			}
			return uint64(m.Events - 1), nil
		}
	}
	return 0, fmt.Errorf("match not found")
}

func referenceBot(t *testing.T, seed uint64) seat.Seat {
	t.Helper()
	b, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, host.MatchSeed(seed, 1)^1, true)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func matchDone(t *testing.T, r *host.Registry, table host.TableID) (bool, string) {
	t.Helper()
	ms, err := r.Matches(table)
	if err != nil {
		return false, ""
	}
	for _, m := range ms {
		if m.Match != 1 {
			continue
		}
		switch m.State {
		case protocol.MatchFinished:
			return true, m.Head
		case protocol.MatchCrashed:
			t.Fatalf("table %s crashed: %s", table, m.Head)
		}
	}
	return false, ""
}

// openSBV2Stream opens the SSE route and decodes each pushed state document.
func openSBV2Stream(t *testing.T, base, path, token string) <-chan stateBody {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		cancel()
		resp.Body.Close()
		t.Fatalf("stream: status %d", resp.StatusCode)
	}
	out := make(chan stateBody, 8)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		br := bufio.NewReader(resp.Body)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			var body stateBody
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				return
			}
			select {
			case out <- body:
			case <-ctx.Done():
				return
			}
		}
	}()
	t.Cleanup(cancel)
	return out
}

func getSBV2State(t *testing.T, base, path, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

func postSBV2Send(t *testing.T, base, path, token, requestID string, candidate int) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"request_id": requestID, "candidate_id": candidate})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}
