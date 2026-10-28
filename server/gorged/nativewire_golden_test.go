package gorged

// TestNativeWireGolden pins the native HTTP/SSE client wire for one fixed-seed
// two-seat game: a human seat (seat 0, HTTP) against the default deterministic
// bot, driven through the same httpapi.Handler server/gorged serves
// (server/gorged/gorged.go mounts it at topMux.Handle("/", httpapi.NewHandler(r, opts))).
//
// The routes pinned here are the registered patterns, not guesses:
//   - GET  /api/tables/{t}/matches/{k}/pending        server/host/httpapi/handler.go
//   - GET  /api/tables/{t}/matches/{k}/view           server/host/httpapi/handler.go
//   - POST /api/tables/{t}/matches/{k}/intent         server/host/httpapi/handler.go
//   - GET  /api/stream                               server/host/httpapi/sse.go
//   - POST /api/subscribe                            server/host/httpapi/handler.go
//
// The golden canonicalises only the two values that are inherently random:
// the SSE session id (minted by crypto/rand) and, with it, the session half of
// each `id: <session>:<frame>` line. Everything else is the exact JSON/SSE
// bytes the handler wrote.
//
// A diff here means the client contract changed. It needs operator sign-off;
// do not regenerate until that sign-off exists.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	nativeWirePath   = "testdata/nativewire/2seat.golden.json.gz"
	nativeWireTable  = "t1"
	nativeWireMatch  = 1
	nativeWireSeat   = 0
	nativeWireToken  = "nativewire-seat-0"
	nativeWireSeed   = 0x5bc0
	nativeWireSteps  = 40
	nativeWireDeckA  = "death-n-taxes"
	nativeWireDeckB  = "mono-red-prowess"
	nativeWireCanonS = "s0"
)

// nativeWireGolden is the gzipped golden's JSON shape. Pending, View and
// Frames hold the exact bytes the handler wrote, as strings so re-marshalling
// the golden cannot renormalise them.
type nativeWireGolden struct {
	Description string           `json:"description"`
	Table       string           `json:"table"`
	Match       int              `json:"match"`
	Seat        int              `json:"seat"`
	Seed        uint64           `json:"seed"`
	Hello       string           `json:"hello"`
	Steps       []nativeWireStep `json:"steps"`
}

type nativeWireStep struct {
	Pending string   `json:"pending"`
	View    string   `json:"view"`
	Frames  []string `json:"frames"`
}

func TestNativeWireGolden(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := config{
		cards:     testutil.CorpusDirOrSkip(t),
		decks:     "../../testkit/testutil/decks",
		dir:       t.TempDir(),
		seed:      nativeWireSeed,
		tables:    1,
		seats:     2,
		humans:    []int{nativeWireSeat},
		mulligans: 0,
		// The deterministic default policy. Search policies arm the host's
		// wall-clock decision budget, which would make the bytes depend on
		// machine load; a golden must not.
		botPolicy: host.BotPolicy,
	}
	cfgs := cfg.tableConfigs(nil, []string{nativeWireDeckA, nativeWireDeckB}, view.Public)
	if len(cfgs) != 1 {
		t.Fatalf("tableConfigs returned %d tables, want 1", len(cfgs))
	}
	r, err := host.New(cfg.hostOptions(reg, deckLoader(reg, cfg.decks)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(cfgs[0]); err != nil {
		t.Fatal(err)
	}
	gate, err := newSeatGate(nativeWireToken, nativeWireTable, []int{nativeWireSeat})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.NewHandler(r, httpapi.Options{
		Seat: gate.resolve,
		// Widgets and keep-alive comments are wall-clock paced. This stream is
		// focus-only (so no widgets) and the test finishes long before either
		// ticker, but parking them out of reach keeps them from ever entering
		// the golden.
		WidgetInterval: time.Hour,
		KeepAlive:      time.Hour,
	}))
	defer srv.Close()

	// Open the stream and subscribe BEFORE the table starts: the hello is then
	// the stable idle table, and the first recorded step begins with the
	// match_start/snapshot pair instead of racing the deal.
	stream, frames := openNativeWireStream(t, srv.URL, gate.token(nativeWireTable, nativeWireSeat))
	defer stream.Body.Close()
	helloRaw := <-frames
	session := nativeWireSession(t, helloRaw)
	hello := nativeWireCanon(helloRaw, session)
	postNativeWire(t, srv.URL+"/api/subscribe", "", protocol.Subscribe{
		Session: session, Table: nativeWireTable, Mode: protocol.ModeFocus,
	})

	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}

	steps := make([]nativeWireStep, 0, nativeWireSteps)
	for len(steps) < nativeWireSteps {
		d, ok := waitNativeWireDecision(t, srv.URL, r)
		if !ok {
			break
		}
		collected, err := collectNativeWireFrames(t, frames, d, session)
		if err != nil {
			t.Fatal(err)
		}
		pendingRaw := getNativeWire(t, fmt.Sprintf("%s/api/tables/%s/matches/%d/pending?seat=%d&token=%s",
			srv.URL, url.PathEscape(nativeWireTable), nativeWireMatch, nativeWireSeat, url.QueryEscape(nativeWireToken)))
		var got decision.Decision
		if err := json.Unmarshal(pendingRaw, &got); err != nil {
			t.Fatalf("pending JSON: %v", err)
		}
		if got.Seq != d.Seq || got.Player != d.Player {
			t.Fatalf("pending moved while collecting frames: wanted seq %d player %d, got %d/%d",
				d.Seq, d.Player, got.Seq, got.Player)
		}
		viewRaw := getNativeWire(t, fmt.Sprintf("%s/api/tables/%s/matches/%d/view?seat=%d&token=%s",
			srv.URL, url.PathEscape(nativeWireTable), nativeWireMatch, nativeWireSeat, url.QueryEscape(nativeWireToken)))
		var v view.View
		if err := json.Unmarshal(viewRaw, &v); err != nil {
			t.Fatalf("view JSON: %v", err)
		}
		if v.Decision == nil || v.Decision.Seq != d.Seq {
			t.Fatalf("view does not carry the pending decision: view decision %+v, pending seq %d", v.Decision, d.Seq)
		}
		steps = append(steps, nativeWireStep{Pending: string(pendingRaw), View: string(viewRaw), Frames: collected})
		answerNativeWire(t, srv.URL, d)
	}

	g := nativeWireGolden{
		Description: "native HTTP/SSE wire, fixed-seed 2-seat human-vs-bot game; canonicalised only the random SSE session id",
		Table:       nativeWireTable,
		Match:       nativeWireMatch,
		Seat:        nativeWireSeat,
		Seed:        nativeWireSeed,
		Hello:       hello,
		Steps:       steps,
	}
	if len(steps) != nativeWireSteps {
		t.Fatalf("recorded %d decisions, want %d: the script's game ended early", len(steps), nativeWireSteps)
	}
	got, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	if os.Getenv("NATIVEWIRE_REGEN") == "1" {
		writeNativeWireGolden(t, got)
		return
	}
	want := readNativeWireGolden(t)
	if !bytes.Equal(got, want) {
		t.Fatalf("native client wire golden mismatch: a diff here means the client contract changed and needs operator sign-off. Do not regenerate until that sign-off exists; then run NATIVEWIRE_REGEN=1 go test -run TestNativeWireGolden ./server/gorged/.\n%s",
			nativeWireDiff(want, got))
	}
}

// openNativeWireStream connects GET /api/stream with the seat token, so the
// session is bound to the table, and returns a channel of canonicalised raw
// SSE messages. Comments (keep-alive pings) are dropped; they are not frames.
func openNativeWireStream(t *testing.T, baseURL, token string) (*http.Response, <-chan string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/stream?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		resp.Body.Close()
		t.Fatalf("GET /api/stream: status %d content-type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	raw := make(chan string, 4096)
	go func() {
		defer close(raw)
		br := bufio.NewReader(resp.Body)
		for {
			msg, err := readNativeWireSSE(br)
			if msg != "" {
				raw <- msg
			}
			if err != nil {
				return
			}
		}
	}()
	return resp, raw
}

// readNativeWireSSE reads one event's raw text, including its trailing blank
// line, and skips comment lines. io.EOF is returned with the final partial
// message, if any.
func readNativeWireSSE(br *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			if strings.HasPrefix(line, ":") {
				// keep-alive comment: not a frame
			} else if line == "\n" || line == "\r\n" {
				if b.Len() > 0 {
					return b.String(), err
				}
			} else {
				b.WriteString(line)
			}
		}
		if err != nil {
			return b.String(), err
		}
	}
}

// nativeWireCanon replaces the only random wire value, the SSE session id,
// everywhere it appears: the hello body and each id: line.
func nativeWireCanon(raw, session string) string {
	return strings.ReplaceAll(raw, session, nativeWireCanonS)
}

func nativeWireSession(t *testing.T, hello string) string {
	t.Helper()
	event, data := nativeWireSSE(t, hello)
	if event != string(protocol.THello) {
		t.Fatalf("first SSE frame is %q, want hello", event)
	}
	var f protocol.Frame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("hello frame JSON: %v", err)
	}
	var h protocol.Hello
	if err := f.Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Session == "" {
		t.Fatal("hello carried no session")
	}
	return h.Session
}

func nativeWireSSE(t *testing.T, raw string) (event string, data []byte) {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = []byte(strings.TrimPrefix(line, "data: "))
		}
	}
	if event == "" || len(data) == 0 {
		t.Fatalf("malformed SSE message: %q", raw)
	}
	return event, data
}

// collectNativeWireFrames drains SSE messages through the decision frame for
// d. fanout pushes a burst's event frames and then its decision frame, so the
// decision frame is the deterministic end of the step.
func collectNativeWireFrames(t *testing.T, frames <-chan string, d *decision.Decision, session string) ([]string, error) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	out := []string{}
	// Decision.Seq is the index of the DecisionAsk event ask() emits after
	// assigning it, so the decision frame carries the same seq.
	wantSeq := d.Seq
	for {
		select {
		case raw, ok := <-frames:
			if !ok {
				return out, fmt.Errorf("SSE stream closed before decision seq %d", d.Seq)
			}
			event, data := nativeWireSSE(t, raw)
			out = append(out, nativeWireCanon(raw, session))
			if event != string(protocol.TDecision) {
				continue
			}
			var f protocol.Frame
			if err := json.Unmarshal(data, &f); err != nil {
				t.Fatalf("decision frame JSON: %v", err)
			}
			var body protocol.DecisionBody
			if err := json.Unmarshal(f.Body, &body); err != nil {
				t.Fatalf("decision frame body: %v", err)
			}
			if f.Seq == wantSeq && body.Player == uint8(d.Player) {
				return out, nil
			}
		case <-deadline:
			return out, fmt.Errorf("timed out waiting for decision frame seq %d player %d", wantSeq, d.Player)
		}
	}
}

// waitNativeWireDecision polls the public pending route until the human seat
// has a decision, or reports that the match finished first.
func waitNativeWireDecision(t *testing.T, baseURL string, r *host.Registry) (*decision.Decision, bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	pendingURL := fmt.Sprintf("%s/api/tables/%s/matches/%d/pending?seat=%d&token=%s",
		baseURL, url.PathEscape(nativeWireTable), nativeWireMatch, nativeWireSeat, url.QueryEscape(nativeWireToken))
	for {
		resp, err := http.Get(pendingURL)
		if err == nil {
			body, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if rerr == nil && resp.StatusCode == http.StatusOK {
				var d decision.Decision
				if err := json.Unmarshal(body, &d); err != nil {
					t.Fatalf("pending JSON: %v", err)
				}
				return &d, true
			}
		}
		if ms, err := r.Matches(host.TableID(nativeWireTable)); err == nil {
			for _, m := range ms {
				if m.Match == nativeWireMatch && m.State != protocol.MatchLive {
					return nil, false
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for a pending decision")
		}
		time.Sleep(time.Millisecond)
	}
}

func answerNativeWire(t *testing.T, baseURL string, d *decision.Decision) {
	t.Helper()
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	// "First legal option": the lowest-indexed option(s), repaired by the same
	// finaliser every bot answer runs so Groups/budgets/Required cannot make
	// the script's pick invalid.
	n := d.Min
	if n == 0 && len(d.Options) > 0 {
		n = 1
	}
	if n > len(d.Options) {
		n = len(d.Options)
	}
	for i := 0; i < n; i++ {
		in.Choices = append(in.Choices, i)
	}
	in = botpolicy.Clamp(d, in)
	if err := d.Validate(in); err != nil {
		t.Fatalf("first-legal-option script produced an invalid answer for %s seq %d: %v (%+v)", d.Kind, d.Seq, err, in)
	}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	postNativeWire(t, fmt.Sprintf("%s/api/tables/%s/matches/%d/intent", baseURL, url.PathEscape(nativeWireTable), nativeWireMatch), nativeWireToken, body)
}

func postNativeWire(t *testing.T, endpoint, token string, body any) {
	t.Helper()
	var raw []byte
	switch v := body.(type) {
	case []byte:
		raw = v
	default:
		var err error
		if raw, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		msg, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: status %d body %s", endpoint, resp.StatusCode, msg)
	}
}

func getNativeWire(t *testing.T, endpoint string) []byte {
	t.Helper()
	resp, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d body %s", endpoint, resp.StatusCode, body)
	}
	return body
}

func writeNativeWireGolden(t *testing.T, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(nativeWirePath), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() > 1<<20 {
		t.Fatalf("%s is %d bytes, over the 1 MB golden budget", nativeWirePath, buf.Len())
	}
	if err := os.WriteFile(nativeWirePath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readNativeWireGolden(t *testing.T) []byte {
	t.Helper()
	if fi, err := os.Stat(nativeWirePath); err == nil && fi.Size() > 1<<20 {
		t.Fatalf("%s is %d bytes, over the 1 MB golden budget", nativeWirePath, fi.Size())
	}
	f, err := os.Open(nativeWirePath)
	if err != nil {
		t.Fatalf("%v (generate with NATIVEWIRE_REGEN=1 go test -run TestNativeWireGolden ./server/gorged/)", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// nativeWireDiff reports the first differing line with a little context, so
// the failure names the contract change without dumping a multi-megabyte log.
func nativeWireDiff(want, got []byte) string {
	wl, gl := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] == gl[i] {
			continue
		}
		lo, hi := i-3, i+4
		if lo < 0 {
			lo = 0
		}
		if hi > len(wl) {
			hi = len(wl)
		}
		if hi > len(gl) {
			hi = len(gl)
		}
		var b strings.Builder
		for j := lo; j < hi; j++ {
			mark := " "
			if j == i {
				mark = "!"
			}
			fmt.Fprintf(&b, "%s want: %s\n", mark, wl[j])
			fmt.Fprintf(&b, "%s got:  %s\n", mark, gl[j])
		}
		return b.String()
	}
	return fmt.Sprintf("line count changed: want %d lines, got %d", len(wl), len(gl))
}
