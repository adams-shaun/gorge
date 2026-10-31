package sbv2http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// humanExt decodes x_gorge_human_v1 from a seat decision.
type humanExt struct {
	Log    []string       `json:"log"`
	Labels map[int]string `json:"labels"`
	Plans  []struct {
		CandidateID int                  `json:"candidate_id"`
		ActionID    string               `json:"action_id"`
		Plan        decision.PaymentPlan `json:"plan"`
	} `json:"plans"`
	CanUndo    bool `json:"can_undo"`
	NativeOnly bool `json:"native_only"`
}

func decodeHumanExt(t *testing.T, sd protocol.SeatDecision) humanExt {
	t.Helper()
	raw, ok := sd.Extensions["x_gorge_human_v1"]
	if !ok {
		t.Fatalf("decision extensions lack x_gorge_human_v1: %v", sd.Extensions)
	}
	var ext humanExt
	if err := json.Unmarshal(raw, &ext); err != nil {
		t.Fatalf("x_gorge_human_v1: %v (%s)", err, raw)
	}
	return ext
}

// isNativeOnly reports whether the request is the synthetic candidate-less
// frame the hub publishes for a decision v2 cannot pose.
func isNativeOnly(d protocol.SeatDecision) bool {
	raw, ok := d.Extensions["x_gorge_human_v1"]
	if !ok {
		return false
	}
	var ext struct {
		NativeOnly bool `json:"native_only"`
	}
	if json.Unmarshal(raw, &ext) != nil {
		return false
	}
	return ext.NativeOnly
}

func startExtensionServer(t *testing.T, reg *cards.Registry, hub *Hub, rec *recorder, table host.TableID, seed uint64) (*host.Registry, string) {
	t.Helper()
	r, err := host.New(sbv2RepoOptions(t, reg, hub, rec))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	cfg := sbv2TableCfg(table, seed)
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		c, ok := map[string]SeatClaim{"tok": {Table: table, Seat: 0}}[strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")]
		return c, ok
	}})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if err := r.Start(table); err != nil {
		t.Fatal(err)
	}
	return r, srv.URL
}

// MatchRewound forwards the host's post-undo engine swap to the wrapped tap.
// recTap is the test's recording wrapper; the production tap is installed
// directly, so this only exists so a wrapped tap sees the same hook.
func (r recTap) MatchRewound(e any) {
	if rw, ok := r.HumanTap.(interface{ MatchRewound(any) }); ok {
		rw.MatchRewound(e)
	}
}

var nativeInLog = regexp.MustCompile(`#[0-9]|answers [a-z_]+:`)

func TestSBV2ExtensionCarriesLogLabelsPlans(t *testing.T) {
	const seed = 3000
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	rec := &recorder{}
	r, base := startExtensionServer(t, reg, hub, rec, "ext", seed)
	key := seatKey{table: "ext", match: 1, seat: 0}
	ss := hub.seat(key)
	p := &picker{bot: referenceBot(t, seed)}
	deadline := time.Now().Add(90 * time.Second)
	nativeSeen := 0
	decisions := 0
	sawLog := false
	sawPlan := false

	for decisions < 200 {
		if done, _ := matchDone(t, r, "ext"); done {
			t.Fatal("game ended before a plan-bearing cast was offered")
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out before a plan-bearing cast was offered")
		}
		if n := ss.nativeOnlyCount(); n > nativeSeen {
			d, err := r.Pending("ext", 1, 0)
			if err == nil {
				seq, herr := sbv2HeadSeq(r, "ext", 1)
				if herr != nil {
					t.Fatal(herr)
				}
				v, verr := r.ViewAtSeat("ext", 1, seq, 0)
				if verr != nil {
					t.Fatal(verr)
				}
				v.Decision = d
				in, derr := p.bot.Decide(t.Context(), v, *d)
				if derr != nil {
					t.Fatal(derr)
				}
				if err := r.SubmitIntent("ext", 1, 0, in); err != nil {
					t.Fatal(err)
				}
				nativeSeen = n
				continue
			}
		}
		status, body := getSBV2State(t, base, "/api/sbv2/v1/tables/ext/matches/1/state", "tok")
		if status != http.StatusOK {
			t.Fatalf("state: %d %q", status, body)
		}
		var doc stateBody
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("state body %q: %v", body, err)
		}
		if doc.Request == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		req, _ := ss.snapshot()
		if req == nil || fmt.Sprint(req.id) != doc.Request.RequestID {
			time.Sleep(time.Millisecond)
			continue
		}
		if isNativeOnly(doc.Request.Decision) {
			time.Sleep(time.Millisecond)
			continue
		}
		decisions++
		if decisions%10 == 1 {
			t.Logf("decision %d kind %s candidates %d log %d", decisions, doc.Request.Decision.Context.Kind, len(doc.Request.Decision.Candidates), len(decodeHumanExt(t, doc.Request.Decision).Log))
		}
		if _, ok := doc.Request.Decision.Extensions["x_gorge_view_v1"]; !ok {
			t.Fatalf("decision %d lacks x_gorge_view_v1: %v", decisions, doc.Request.Decision.Extensions)
		}
		var payload xview.Payload
		if err := json.Unmarshal(doc.Request.Decision.Extensions["x_gorge_view_v1"], &payload); err != nil {
			t.Fatalf("x_gorge_view_v1: %v", err)
		}
		ext := decodeHumanExt(t, doc.Request.Decision)
		if ext.NativeOnly {
			t.Fatalf("a v2 choose request reported native_only")
		}
		if len(ext.Log) > 0 {
			sawLog = true
		}
		for _, line := range ext.Log {
			if nativeInLog.MatchString(line) {
				t.Fatalf("decision %d: log line carries a native id: %q", decisions, line)
			}
		}
		if len(ext.Labels) != len(doc.Request.Decision.Candidates) {
			t.Fatalf("decision %d: %d labels for %d candidates", decisions, len(ext.Labels), len(doc.Request.Decision.Candidates))
		}
		for _, c := range doc.Request.Decision.Candidates {
			label, ok := ext.Labels[int(c.CandidateID)]
			if !ok || label == "" {
				t.Fatalf("decision %d candidate %d has label %q (present %v)", decisions, c.CandidateID, label, ok)
			}
		}
		if len(ext.Plans) > 0 {
			for _, pl := range ext.Plans {
				if pl.CandidateID < 0 || pl.CandidateID >= len(doc.Request.Decision.Candidates) {
					t.Fatalf("plan names candidate %d of %d", pl.CandidateID, len(doc.Request.Decision.Candidates))
				}
				if pl.ActionID == "" || pl.Plan.ID == "" {
					t.Fatalf("plan is not identified: %+v", pl)
				}
			}
			sawPlan = true
			break
		}
		cand, miss := p.pick(req.ask)
		if miss != "" {
			t.Logf("seq %d: production bot pick missed candidates (%s); answering 0", req.ask.Pose.Native.Seq, miss)
			cand = 0
		}
		if status, body := postSBV2Send(t, base, "/api/sbv2/v1/tables/ext/matches/1/send", "tok", doc.Request.RequestID, cand); status != http.StatusNoContent {
			t.Fatalf("send request %s candidate %d: %d %q", doc.Request.RequestID, cand, status, body)
		}
	}
	if !sawPlan {
		t.Fatal("no plan-bearing cast appeared")
	}
	if !sawLog {
		t.Fatal("no extension carried log lines")
	}
}

func awaitExtensionRequest(t *testing.T, base string, table host.TableID, ss *seatState, p *picker, pred func(stateBody) bool) (stateBody, []byte) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	path := "/api/sbv2/v1/tables/" + string(table) + "/matches/1/state"
	for {
		if time.Now().After(deadline) {
			status, body := getSBV2State(t, base, path, "tok")
			req, _ := ss.snapshot()
			id := uint64(0)
			if req != nil {
				id = req.id
			}
			t.Fatalf("timed out waiting for a v2 request: state %d %q, current %d, nativeOnly %d", status, body, id, ss.nativeOnlyCount())
		}
		status, body := getSBV2State(t, base, path, "tok")
		if status != http.StatusOK {
			t.Fatalf("state: %d %q", status, body)
		}
		var doc stateBody
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("state body %q: %v", body, err)
		}
		if doc.Request == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		req, _ := ss.snapshot()
		if req == nil || fmt.Sprint(req.id) != doc.Request.RequestID {
			time.Sleep(time.Millisecond)
			continue
		}
		if isNativeOnly(doc.Request.Decision) {
			time.Sleep(time.Millisecond)
			continue
		}
		if pred == nil || pred(doc) {
			return doc, body
		}
		cand, miss := p.pick(req.ask)
		if miss != "" {
			cand = 0
		}
		if status, body := postSBV2Send(t, base, path[:len(path)-len("/state")]+"/send", "tok", doc.Request.RequestID, cand); status != http.StatusNoContent {
			t.Fatalf("send request %s candidate %d: %d %q", doc.Request.RequestID, cand, status, body)
		}
	}
}

func postHumanAction(t *testing.T, base, path, token, requestID string, action map[string]bool) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"request_id": requestID, "x_gorge_human_v1": action})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(string(raw)))
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

func matchHead(t *testing.T, r *host.Registry, table host.TableID) (string, int) {
	t.Helper()
	ms, err := r.Matches(table)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.Match == 1 {
			return m.Head, m.Events
		}
	}
	t.Fatalf("match 1 of table %s not found", table)
	return "", 0
}

func TestSBV2ExtensionConcede(t *testing.T) {
	const seed = 3000
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	rec := &recorder{}
	r, base := startExtensionServer(t, reg, hub, rec, "concede", seed)
	ss := hub.seat(seatKey{table: "concede", match: 1, seat: 0})
	p := &picker{bot: referenceBot(t, seed)}
	doc, _ := awaitExtensionRequest(t, base, "concede", ss, p, func(d stateBody) bool {
		return d.Request.Decision.Context.Kind == "priority"
	})
	if status, body := postHumanAction(t, base, "/api/sbv2/v1/tables/concede/matches/1/send", "tok", doc.Request.RequestID, map[string]bool{"concede": true}); status != http.StatusNoContent {
		t.Fatalf("concede: %d %q", status, body)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if done, _ := matchDone(t, r, "concede"); done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("match did not finish after concede")
		}
		time.Sleep(time.Millisecond)
	}
	evs, err := r.Events("concede", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range evs {
		if ev.Event.Kind == events.PlayerLost.String() && ev.Event.Player == 0 && ev.Event.Text == "conceded" {
			return
		}
	}
	t.Fatalf("no conceded PlayerLost for seat 0 in %d events", len(evs))
}

func TestSBV2ExtensionUndo(t *testing.T) {
	const seed = 3000
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	rec := &recorder{}
	r, base := startExtensionServer(t, reg, hub, rec, "undo", seed)
	ss := hub.seat(seatKey{table: "undo", match: 1, seat: 0})
	p := &picker{bot: referenceBot(t, seed)}
	doc, _ := awaitExtensionRequest(t, base, "undo", ss, p, func(d stateBody) bool {
		return d.Request.Decision.Context.Kind == "priority"
	})
	_, beforeEvents := matchHead(t, r, "undo")
	pass := -1
	for _, c := range doc.Request.Decision.Candidates {
		if c.Semantic.Kind == "pass" {
			pass = int(c.CandidateID)
			break
		}
	}
	if pass < 0 {
		t.Fatal("priority request offers no pass candidate")
	}
	if status, body := postSBV2Send(t, base, "/api/sbv2/v1/tables/undo/matches/1/send", "tok", doc.Request.RequestID, pass); status != http.StatusNoContent {
		t.Fatalf("pass: %d %q", status, body)
	}
	doc, _ = awaitExtensionRequest(t, base, "undo", ss, p, func(d stateBody) bool {
		return decodeHumanExt(t, d.Request.Decision).CanUndo
	})
	_, preUndoEvents := matchHead(t, r, "undo")
	if status, body := postHumanAction(t, base, "/api/sbv2/v1/tables/undo/matches/1/send", "tok", doc.Request.RequestID, map[string]bool{"undo": true}); status != http.StatusNoContent {
		t.Fatalf("undo: %d %q", status, body)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, evs := matchHead(t, r, "undo")
		if evs == beforeEvents {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("undo did not rewind: %d events, want %d", evs, beforeEvents)
		}
		time.Sleep(time.Millisecond)
	}
	if preUndoEvents <= beforeEvents {
		t.Fatalf("pre-undo log %d events, before pass %d", preUndoEvents, beforeEvents)
	}
	doc, _ = awaitExtensionRequest(t, base, "undo", ss, p, nil)
	if doc.Request.Decision.Context.Kind != "priority" {
		t.Fatalf("after undo the re-posed decision is %q, want priority", doc.Request.Decision.Context.Kind)
	}
	if !decodeHumanExt(t, doc.Request.Decision).CanUndo {
		t.Fatal("can_undo should still be true for the earlier human intent")
	}
}

func TestSBV2PrivacyNoOpponentHand(t *testing.T) {
	const seed = 3000
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	rec := &recorder{}
	r, base := startExtensionServer(t, reg, hub, rec, "privacy", seed)
	ss := hub.seat(seatKey{table: "privacy", match: 1, seat: 0})
	p := &picker{bot: referenceBot(t, seed)}
	nativeSeen := 0
	decisions := 0
	sawHidden := false
	for decisions < 20 {
		if n := ss.nativeOnlyCount(); n > nativeSeen {
			d, err := r.Pending("privacy", 1, 0)
			if err == nil {
				seq, herr := sbv2HeadSeq(r, "privacy", 1)
				if herr != nil {
					t.Fatal(herr)
				}
				v, verr := r.ViewAtSeat("privacy", 1, seq, 0)
				if verr != nil {
					t.Fatal(verr)
				}
				v.Decision = d
				in, derr := p.bot.Decide(t.Context(), v, *d)
				if derr != nil {
					t.Fatal(derr)
				}
				if err := r.SubmitIntent("privacy", 1, 0, in); err != nil {
					t.Fatal(err)
				}
				nativeSeen = n
				continue
			}
		}
		doc, body := awaitExtensionRequest(t, base, "privacy", ss, p, nil)
		seq, err := sbv2HeadSeq(r, "privacy", 1)
		if err != nil {
			t.Fatal(err)
		}
		omniscient, err := r.ViewAt("privacy", 1, seq)
		if err != nil {
			t.Fatal(err)
		}
		if len(omniscient.Players) < 2 {
			t.Fatalf("omniscient view has %d players", len(omniscient.Players))
		}
		var payload xview.Payload
		if err := json.Unmarshal(doc.Request.Decision.Extensions["x_gorge_view_v1"], &payload); err != nil {
			t.Fatalf("x_gorge_view_v1: %v", err)
		}
		if payload.View.Players[1].Hand != nil {
			t.Fatalf("decision %d: x_gorge_view_v1 leaks opponent hand: %v", decisions+1, payload.View.Players[1].Hand)
		}
		ext := decodeHumanExt(t, doc.Request.Decision)
		public := map[string]bool{}
		for _, c := range omniscient.Players[0].Hand {
			public[c.Name] = true
		}
		for i := range omniscient.Players {
			pv := &omniscient.Players[i]
			for _, c := range pv.Battlefield {
				public[c.Name] = true
			}
			for _, c := range pv.Graveyard {
				public[c.Name] = true
			}
			for _, c := range pv.Exile {
				public[c.Name] = true
			}
			for _, c := range pv.Command {
				public[c.Name] = true
			}
			for _, c := range pv.Commanders {
				public[c.Name] = true
			}
			for _, c := range pv.PlanarDeck {
				public[c.Name] = true
			}
			if pv.LibraryTop != nil {
				public[pv.LibraryTop.Name] = true
			}
		}
		for _, st := range omniscient.Stack {
			public[st.Name] = true
		}
		for _, card := range omniscient.Players[1].Hand {
			sawHidden = true
			if public[card.Name] {
				continue
			}
			if strings.Contains(string(body), card.Name) {
				t.Fatalf("decision %d: response body leaks opponent hand card %q", decisions+1, card.Name)
			}
			if card.ID >= 1000 {
				id := fmt.Sprintf("\"id\":%d", card.ID)
				if strings.Contains(string(body), id) {
					t.Fatalf("decision %d: response body leaks opponent hand object id %d", decisions+1, card.ID)
				}
			}
			for _, line := range ext.Log {
				if strings.Contains(line, card.Name) {
					t.Fatalf("decision %d: log leaks opponent hand card %q: %q", decisions+1, card.Name, line)
				}
			}
		}
		req, _ := ss.snapshot()
		cand, miss := p.pick(req.ask)
		if miss != "" {
			cand = 0
		}
		if status, b := postSBV2Send(t, base, "/api/sbv2/v1/tables/privacy/matches/1/send", "tok", doc.Request.RequestID, cand); status != http.StatusNoContent {
			t.Fatalf("send request %s candidate %d: %d %q", doc.Request.RequestID, cand, status, b)
		}
		decisions++
	}
	if !sawHidden {
		t.Fatal("no opponent hand card was observable across 20 decisions")
	}
}

func TestSBV2ExtensionNativeOnly(t *testing.T) {
	hub := NewHub()
	ss := hub.seat(seatKey{table: "nativeonly", match: 1, seat: 0})
	ss.setExtension(newExtensionBuilder())
	ss.NativeOnly(humanseat.NativeOnly{Seat: 0, GameID: "g-1"})
	req, _ := ss.snapshot()
	if req == nil {
		t.Fatal("native-only decision published no request")
	}
	if !req.nativeOnly {
		t.Fatal("native-only request is not marked native_only")
	}
	if ss.open(req.id) {
		t.Fatal("native-only request is answerable through send")
	}
	if ss.answer(req.id, 0) {
		t.Fatal("send answered a native-only request")
	}
	ext := decodeHumanExt(t, req.ask.Decision)
	if !ext.NativeOnly {
		t.Fatal("x_gorge_human_v1 lacks native_only")
	}
	if len(ext.Labels) != 0 || len(ext.Plans) != 0 {
		t.Fatalf("native-only extension carries labels %v plans %v", ext.Labels, ext.Plans)
	}
	if _, ok := req.ask.Decision.Extensions["x_gorge_view_v1"]; !ok {
		t.Fatal("native-only request lacks x_gorge_view_v1")
	}
}
