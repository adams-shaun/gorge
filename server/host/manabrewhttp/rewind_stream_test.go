//go:build manabrew

package manabrewhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// gatedWriter is a stream's ResponseWriter whose Flush blocks until the test
// opens the gate, so the frames of several engine bursts pile up in the
// session and reach the stream as one wake-up.
type gatedWriter struct {
	mu   sync.Mutex
	hdr  http.Header
	buf  bytes.Buffer
	gate chan struct{}
}

func (w *gatedWriter) Header() http.Header { return w.hdr }
func (w *gatedWriter) WriteHeader(int)     {}
func (w *gatedWriter) Flush()              { <-w.gate }

func (w *gatedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// prompts decodes every PromptMessage written so far.
func (w *gatedWriter) prompts(t *testing.T) []mb.PromptMessage {
	t.Helper()
	w.mu.Lock()
	raw := w.buf.String()
	w.mu.Unlock()
	var out []mb.PromptMessage
	for _, ev := range strings.Split(raw, "\n\n") {
		data, ok := strings.CutPrefix(strings.TrimSpace(ev), "data: ")
		if !ok {
			continue
		}
		var msg mb.EngineMessage
		if _, err := mb.Decode([]byte(data), &msg); err != nil {
			t.Fatalf("decoding %q: %v", data, err)
		}
		if pm, ok := msg.Value.(mb.PromptMessage); ok {
			out = append(out, pm)
		}
	}
	return out
}

// passOrFirst answers d with its pass option, else its first Min options.
func passOrFirst(d *decision.Decision) decision.Intent {
	for _, o := range d.Options {
		if o.Kind == "pass" {
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
		}
	}
	return legalIntent(d)
}

// TestStreamResendsPromptAnsweredThenRewound pins the undo of a just-answered
// prompt over the stream. The seat answers its open prompt and an undo puts
// the same decision back before the stream has refreshed, so the answer's
// burst and the rewind reach the stream as one wake-up. The rewound decision's
// seq is the promptId the stream last sent, and the client has withdrawn that
// prompt as answered: the stream must send it again, or the seat waits on a
// decision it can no longer see while the match waits on the seat.
func TestStreamResendsPromptAnsweredThenRewound(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "T1", Seats: 2, Decks: []string{"a", "b"}, Seed: 7,
		Spectator: view.Omniscient, Humans: []int{0}, AutoMana: true}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	_, mux := newHandler(r, Options{Seat: func(*http.Request) (SeatClaim, bool) {
		return SeatClaim{Table: "t1", Seat: 0}, true
	}})
	d := waitForPending(t, r, "t1", 1, 0)

	w := &gatedWriter{hdr: http.Header{}, gate: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t1/matches/1/stream", nil).WithContext(ctx))
	}()
	gateOpen := false
	defer func() {
		cancel()
		if !gateOpen {
			close(w.gate)
		}
		<-done
	}()

	deadline := time.Now().Add(10 * time.Second)
	for len(w.prompts(t)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the stream never sent the open prompt")
		}
		time.Sleep(time.Millisecond)
	}
	if got := w.prompts(t)[0].PromptID; got != int64(d.Seq) {
		t.Fatalf("first prompt %d, want the parked decision %d", got, d.Seq)
	}

	// The stream now sits in Flush. Answer the prompt, then undo that answer
	// as soon as the log holds it, and wait for the rewind to land.
	if err := r.SubmitIntent("t1", 1, 0, passOrFirst(d)); err != nil {
		t.Fatal(err)
	}
	for {
		err := r.Undo("t1", 1, 0)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("undo never accepted: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	for {
		if p, err := r.Pending("t1", 1, state.PlayerID(0)); err == nil && p.Seq == d.Seq {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the rewind never put decision %d back", d.Seq)
		}
		time.Sleep(time.Millisecond)
	}

	close(w.gate)
	gateOpen = true
	for {
		if ps := w.prompts(t); len(ps) >= 2 {
			if got := ps[len(ps)-1].PromptID; got != int64(d.Seq) {
				t.Fatalf("after the rewind the stream sent prompt %d, want %d", got, d.Seq)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stream never re-sent rewound prompt %d (sent %d prompts)", d.Seq, len(w.prompts(t)))
		}
		time.Sleep(time.Millisecond)
	}
}
