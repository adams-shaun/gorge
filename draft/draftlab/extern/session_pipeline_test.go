package extern

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/engine/decision"
)

// TestPipelinedHelloAndErrorKillsSession is the regression for the r2
// finding: the handshake must be read from the session's own reader, so an
// agent that pipelines its hello and its next message (an error) in one
// coalesced write keeps both lines. Before the fix the handshake read went
// through a throwaway *wire.Reader whose bufio buffer swallowed the
// pipelined second line: the session stayed alive, the error line was
// never seen, and the agent then answered the batch normally — the call
// returned an answer instead of failing ErrAgentGone.
func TestPipelinedHelloAndErrorKillsSession(t *testing.T) {
	toAgentR, toAgentW := io.Pipe()
	fromAgentR, fromAgentW := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = toAgentR.Close(); _ = fromAgentW.Close() }()
		rd := wire.NewReader(toAgentR, 0)
		wr := wire.NewWriter(fromAgentW)
		hello, err := rd.Read()
		if err != nil {
			return
		}
		if err := hello.Validate(wire.FromGorge); err != nil || hello.Type != wire.THello {
			return
		}
		// ONE Write, two lines: hello then an agent error. io.Pipe is
		// unbuffered, so gorge's handshake bufio.Reader pulls both lines
		// into its buffer and a throwaway handshake reader drops the
		// second.
		var buf []byte
		for _, m := range []wire.Message{
			validAgentHello("pipeline"),
			{V: wire.Version, Type: wire.TError, Code: "boom", Message: "the agent refuses"},
		} {
			b, err := json.Marshal(m)
			if err != nil {
				return
			}
			buf = append(buf, b...)
			buf = append(buf, '\n')
		}
		if _, err := fromAgentW.Write(buf); err != nil {
			return
		}
		// Stay alive: serve the next batch (only reachable when the error
		// line was lost) so the broken shape fails with a delivered answer
		// instead of hanging the test.
		m, err := rd.Read()
		if err != nil || m.Type != wire.TBatch {
			return
		}
		ans := make([]wire.Answer, 0, len(m.Items))
		for _, it := range m.Items {
			ans = append(ans, wire.Answer{ID: it.ID, Intent: &decision.Intent{Choices: []int{0}}})
		}
		_ = wr.Write(wire.Message{V: wire.Version, Type: wire.TResult, Batch: m.Batch, Answers: ans})
	}()
	a, err := DialConn(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, Options{})
	if err != nil {
		t.Fatalf("dial: %v", err) // precondition: the handshake itself succeeds
	}
	t.Cleanup(func() { _ = a.Close() })
	errCh := make(chan error, 1)
	callGame(a, context.Background(), nil, "u1.0", errCh)
	err = <-errCh
	<-done
	if err == nil {
		t.Fatal("call returned an answer; the pipelined agent error was lost, want the session killed")
	}
	if !errors.Is(err, ErrAgentGone) {
		t.Fatalf("call error %v, want ErrAgentGone (the pipelined error kills the session)", err)
	}
	if !a.Stats().Dead {
		t.Error("Stats().Dead = false after the pipelined agent error")
	}
}
