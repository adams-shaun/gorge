package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

type sseFrame struct {
	id    string
	event string
	frame protocol.Frame
}

// readSSE parses frames until n have arrived or the stream ends.
func readSSE(t *testing.T, body io.Reader, n int) []sseFrame {
	t.Helper()
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var out []sseFrame
	cur := sseFrame{}
	var data bytes.Buffer
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if data.Len() > 0 {
				if err := json.Unmarshal(data.Bytes(), &cur.frame); err != nil {
					t.Fatalf("bad frame data %q: %v", data.String(), err)
				}
				out = append(out, cur)
				if len(out) == n {
					return out
				}
			}
			cur, data = sseFrame{}, bytes.Buffer{}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "id: "):
			cur.id = line[4:]
		case strings.HasPrefix(line, "event: "):
			cur.event = line[7:]
		case strings.HasPrefix(line, "data: "):
			data.WriteString(line[6:])
		}
	}
	return out
}

// pausedServer builds a registry whose table waits on `gate` at every
// decision so the test controls pacing, and serves it.
func pausedServer(t *testing.T, o Options, gate chan struct{}, ring int) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Ring: ring, Sleep: func(time.Duration) {
		if gate != nil {
			<-gate
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 5, Spectator: view.Omniscient}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

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

func TestStreamOpensWithHelloThenSnapshotThenEvents(t *testing.T) {
	gate := make(chan struct{})
	srv, r := pausedServer(t, Options{}, gate, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	defer resp.Body.Close()
	first := readSSE(t, resp.Body, 1)
	if first[0].event != "hello" || first[0].id != "" {
		t.Fatalf("first frame %+v", first[0])
	}
	var hello protocol.Hello
	_ = first[0].frame.Decode(&hello)
	if hello.Session == "" || len(hello.Tables) != 1 {
		t.Fatalf("hello %+v", hello)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: hello.Session, Table: "t1", Mode: protocol.ModeFocus}); code != 204 {
		t.Fatalf("subscribe %d", code)
	}
	_ = r.Start("t1")
	// Only match_start and the focus snapshot are guaranteed before the
	// gate gives up its first decision (the paused table's Sleep hook
	// blocks right after fanning out intent #1's own events, and how many
	// of those land before the ring cap is deck-dependent) — so check the
	// PL-6 ids on exactly those two before releasing the gate for good.
	frames := readSSE(t, resp.Body, 2)
	if frames[0].event != "match_start" || frames[1].event != "snapshot" {
		t.Fatalf("after start: %s, %s", frames[0].event, frames[1].event)
	}
	if frames[1].id != hello.Session+":"+"2" {
		t.Fatalf("snapshot id %q (PL-6 expects <session>:<frame>)", frames[1].id)
	}
	// Release the gate for good and check event seqs stay contiguous over
	// a long stretch of the match.
	close(gate)
	more := readSSE(t, resp.Body, 200)
	var last uint64
	for _, f := range more {
		if f.event == "event" {
			if last != 0 && f.frame.Seq != last+1 {
				t.Fatalf("event seq %d after %d", f.frame.Seq, last)
			}
			last = f.frame.Seq
		}
	}
	if last == 0 {
		t.Fatal("no event frames")
	}
}

func TestLastEventIDResumesExactlyTheMissedFrames(t *testing.T) {
	srv, r := pausedServer(t, Options{ResumeGrace: 5 * time.Second}, nil, 100000)
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	hello := readSSE(t, resp.Body, 1)[0]
	var h protocol.Hello
	_ = hello.frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "t1", Mode: protocol.ModeFocus})
	_ = r.Start("t1")
	got := readSSE(t, resp.Body, 30)
	lastID := got[len(got)-1].id
	cancel() // client drops mid-stream
	resp.Body.Close()
	r.Wait("t1") // the match finishes while we are away; the ring keeps the tail

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	resp2 := openStream(t, ctx2, srv.URL, lastID)
	defer resp2.Body.Close()
	resumed := readSSE(t, resp2.Body, 5)
	if resumed[0].event == "hello" {
		t.Fatalf("resume within the ring started over with hello")
	}
	lastNum, err := strconv.ParseUint(strings.TrimPrefix(lastID, h.Session+":"), 10, 64)
	if err != nil {
		t.Fatalf("last id %q: %v", lastID, err)
	}
	if resumed[0].frame.ID != lastNum+1 {
		t.Fatalf("first resumed frame id %d, want %d", resumed[0].frame.ID, lastNum+1)
	}
	for i := 1; i < len(resumed); i++ {
		if resumed[i].frame.ID != resumed[i-1].frame.ID+1 {
			t.Fatalf("resumed ids not contiguous: %d after %d", resumed[i].frame.ID, resumed[i-1].frame.ID)
		}
	}
}

func TestAnIDOlderThanTheRingStartsOverWithHello(t *testing.T) {
	srv, r := pausedServer(t, Options{}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "t1", Mode: protocol.ModeFocus})
	_ = r.Start("t1")
	r.Wait("t1")
	cancel()
	resp.Body.Close()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	resp2 := openStream(t, ctx2, srv.URL, h.Session+":1")
	defer resp2.Body.Close()
	first := readSSE(t, resp2.Body, 1)[0]
	var h2 protocol.Hello
	if first.event != "hello" || first.frame.Decode(&h2) != nil || h2.Session == h.Session {
		t.Fatalf("expected a fresh hello with a new session, got %s %+v", first.event, h2)
	}
	resp3 := openStream(t, ctx2, srv.URL, "s999:5")
	defer resp3.Body.Close()
	if f := readSSE(t, resp3.Body, 1)[0]; f.event != "hello" {
		t.Fatalf("unknown session resume: %s", f.event)
	}
}

func TestWidgetsAreCoalescedToTheTicker(t *testing.T) {
	srv, r := pausedServer(t, Options{WidgetInterval: 40 * time.Millisecond}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "*", Mode: protocol.ModeOverview})
	start := time.Now()
	_ = r.Start("t1")
	r.Wait("t1")
	elapsed := time.Since(start)
	// Read for 300ms after the match ends, then count what arrived.
	deadline := time.After(300 * time.Millisecond)
	widgets, withID := 0, 0
	done := make(chan []sseFrame)
	go func() { done <- readSSE(t, resp.Body, 1000) }()
	select {
	case fs := <-done:
		for _, f := range fs {
			if f.event == "widget" {
				widgets++
				if f.id != "" {
					withID++
				}
			}
		}
	case <-deadline:
		cancel()
		fs := <-done
		for _, f := range fs {
			if f.event == "widget" {
				widgets++
				if f.id != "" {
					withID++
				}
			}
		}
	}
	ms, _ := r.Matches("t1")
	if widgets == 0 || withID != 0 {
		t.Fatalf("%d widgets, %d with ids", widgets, withID)
	}
	maxTicks := int(elapsed/(40*time.Millisecond)) + 10
	if widgets > maxTicks {
		t.Fatalf("%d widgets for a %v match with a 40ms ticker (%d decisions)", widgets, elapsed, ms[0].Events)
	}
}

func TestStaticServesTheClientWithSPAFallbackOr503(t *testing.T) {
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>gorge</title>")}, "assets/app.js": {Data: []byte("console.log(1)")}}
	srv, _ := pausedServer(t, Options{Web: web}, nil, 0)
	for _, p := range []string{"/", "/t/t1", "/t/t1/m/3"} {
		resp, _ := http.Get(srv.URL + p)
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || !strings.Contains(string(body), "<title>gorge</title>") {
			t.Fatalf("%s: %d %q", p, resp.StatusCode, body)
		}
	}
	resp, _ := http.Get(srv.URL + "/assets/app.js")
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("asset: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, _ = http.Get(srv.URL + "/api/nope")
	if resp.StatusCode != 404 || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("api 404: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	srv2, _ := pausedServer(t, Options{}, nil, 0)
	resp, _ = http.Get(srv2.URL + "/")
	if resp.StatusCode != 503 {
		t.Fatalf("no web build: %d", resp.StatusCode)
	}
}

// TestKeepAlivePingsWhenNoOtherFrameIsDue checks the exact keep-alive wire
// format: a bare ": ping" comment line, on its own SSE record, with
// nothing subscribed to generate real traffic. readSSE can't see it (a
// comment carries no data: line, so it never completes a frame), so this
// scans raw lines instead.
func TestKeepAlivePingsWhenNoOtherFrameIsDue(t *testing.T) {
	srv, _ := pausedServer(t, Options{KeepAlive: 15 * time.Millisecond}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	defer resp.Body.Close()
	readSSE(t, resp.Body, 1) // hello
	sc := bufio.NewScanner(resp.Body)
	seen := 0
	for seen < 3 {
		if !sc.Scan() {
			t.Fatalf("stream ended waiting for pings (err=%v)", sc.Err())
		}
		if sc.Text() == ": ping" {
			seen++
		}
	}
}

// TestOverflowClosesTheStreamWithATerminalFrame forces a session whose
// client never keeps up: a ring cap of 8 (Ring: 1) is far smaller than a
// full match's frame count, so it reliably overflows regardless of
// scheduling. The handler must close with exactly one id-less overflow
// frame and drop the session from the registry.
func TestOverflowClosesTheStreamWithATerminalFrame(t *testing.T) {
	srv, r := pausedServer(t, Options{}, nil, 1) // cap(out) = Ring*8 = 8
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	defer resp.Body.Close()
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "t1", Mode: protocol.ModeFocus}); code != 204 {
		t.Fatalf("subscribe: %d", code)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	frames := readSSE(t, resp.Body, 1<<20) // reads until the server closes the stream
	if len(frames) == 0 {
		t.Fatal("no frames")
	}
	last := frames[len(frames)-1]
	if last.event != "overflow" || last.id != "" {
		t.Fatalf("last frame %+v, want a terminal id-less overflow", last)
	}
	var ov protocol.Overflow
	if err := last.frame.Decode(&ov); err != nil || ov.Dropped == 0 {
		t.Fatalf("overflow body %+v, err=%v", ov, err)
	}
	for i, f := range frames[:len(frames)-1] {
		if f.event == "overflow" {
			t.Fatalf("overflow frame at %d, not just the last (%d total)", i, len(frames))
		}
	}
	if _, ok := r.Session(h.Session); ok {
		t.Fatal("overflowed session is still registered")
	}
}

// TestDisconnectGraceClosesTheSessionAfterItExpires and
// TestReconnectWithinGraceCancelsTheTimer synchronise on the session's own
// Out() channel (closed exactly when CloseSession runs) rather than
// sleeping a guessed duration: the only way Out() can close in either test
// is the grace timer, since nothing else ever pushes to or closes it.

func TestDisconnectGraceClosesTheSessionAfterItExpires(t *testing.T) {
	srv, r := pausedServer(t, Options{ResumeGrace: 10 * time.Millisecond}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	sess, ok := r.Session(h.Session)
	if !ok {
		t.Fatal("session missing right after hello")
	}
	cancel()
	resp.Body.Close()
	select {
	case _, open := <-sess.Out():
		if open {
			t.Fatal("Out() delivered a frame instead of closing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("grace never closed the session")
	}
	if _, ok := r.Session(h.Session); ok {
		t.Fatal("session still registered after grace expired")
	}
}

func TestReconnectWithinGraceCancelsTheTimer(t *testing.T) {
	srv, r := pausedServer(t, Options{ResumeGrace: 30 * time.Millisecond}, nil, 100000)
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	sess, ok := r.Session(h.Session)
	if !ok {
		t.Fatal("session missing right after hello")
	}
	cancel()
	resp.Body.Close()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	resp2 := openStream(t, ctx2, srv.URL, h.Session+":0")
	defer resp2.Body.Close()

	select {
	case _, open := <-sess.Out():
		if !open {
			t.Fatal("session closed despite reconnecting inside the grace window")
		}
	case <-time.After(150 * time.Millisecond): // 5x the grace: it would have fired by now
	}
	if _, ok := r.Session(h.Session); !ok {
		t.Fatal("session was removed despite reconnecting within grace")
	}
}
