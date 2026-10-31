package extern

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// TestDialConnHandshake checks the handshake's product: the refagent's
// hello is carried verbatim and Ref names the agent as ext:<agent>.
func TestDialConnHandshake(t *testing.T) {
	a := dialRefagent(t, newHub(), refagent.Config{}, Options{})
	h := a.Hello()
	if h.Agent != "random" {
		t.Errorf("hello agent = %q, want %q (the default policy name)", h.Agent, "random")
	}
	if h.ModelVersion != "1" {
		t.Errorf("hello model_version = %q, want %q", h.ModelVersion, "1")
	}
	if len(h.Kinds) != 3 || h.Kinds[0] != wire.KindPick || h.Kinds[1] != wire.KindBuild || h.Kinds[2] != wire.KindDecide {
		t.Errorf("hello kinds = %v, want all three", h.Kinds)
	}
	if got := a.Ref(); got.Name != "ext:random" || got.Version != "1" {
		t.Errorf("Ref() = %+v, want {ext:random 1}", got)
	}
	if a.maxBatch != 256 {
		t.Errorf("maxBatch = %d, want the 256 default (no option, no hello max_batch)", a.maxBatch)
	}
}

// TestHandshakeRejects feeds a fake agent's bad first line and expects
// ErrHandshake naming the offending field, with the session closed.
func TestHandshakeRejects(t *testing.T) {
	cases := []struct {
		name  string
		line  wire.Message
		field string
	}{
		{"wrong protocol", wire.Message{V: wire.Version, Type: wire.THello, Protocol: "other/1",
			Agent: "x", ModelVersion: "1", Kinds: []string{wire.KindDecide}}, "protocol"},
		{"bad agent name", wire.Message{V: wire.Version, Type: wire.THello, Protocol: wire.Protocol,
			Agent: "Bad Name", ModelVersion: "1", Kinds: []string{wire.KindDecide}}, "agent"},
		{"unknown view_omit", wire.Message{V: wire.Version, Type: wire.THello, Protocol: wire.Protocol,
			Agent: "x", ModelVersion: "1", Kinds: []string{wire.KindDecide}, ViewOmit: []string{"decision"}}, "view_omit"},
		{"result instead of hello", wire.Message{V: wire.Version, Type: wire.TResult, Batch: 1,
			Answers: []wire.Answer{{ID: 1, Pick: "x"}}}, "type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toAgentR, toAgentW := io.Pipe()
			fromAgentR, fromAgentW := io.Pipe()
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() { _ = toAgentR.Close(); _ = fromAgentW.Close() }()
				wr := wire.NewWriter(fromAgentW)
				rd := wire.NewReader(toAgentR, 0)
				hello, err := rd.Read()
				if err != nil {
					return
				}
				if err := hello.Validate(wire.FromGorge); err != nil || hello.Type != wire.THello {
					return
				}
				_ = wr.Write(tc.line)
			}()
			_, err := DialConn(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, Options{})
			<-done
			if err == nil {
				t.Fatal("DialConn succeeded, want ErrHandshake")
			}
			if !errors.Is(err, ErrHandshake) {
				t.Fatalf("error %v does not wrap ErrHandshake", err)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("error %q does not name the field %q", err.Error(), tc.field)
			}
		})
	}
}

// TestAgentDeathFailsPending: a refagent told to die answers the first
// call and is still alive when that call returns (it only dies when the
// next batch arrives), and every later request — pending, queued or
// fresh — fails with ErrAgentGone; the agent's Stats go Dead.
func TestAgentDeathFailsPending(t *testing.T) {
	a := dialRefagent(t, newHub(), refagent.Config{DieAfter: 1}, Options{})
	errCh := make(chan error, 4)
	callGame(a, context.Background(), nil, "u1.0", errCh)
	wantAnswer(t, errCh, "first call")
	if a.Stats().Dead {
		t.Fatal("Stats().Dead = true after a healthy first call")
	}
	// The agent only dies when the second call's batch arrives, and dies
	// before answering it: the second call fails with ErrAgentGone.
	callGame(a, context.Background(), nil, "u2.0", errCh)
	if err := <-errCh; !errors.Is(err, ErrAgentGone) {
		t.Fatalf("second call error %v, want ErrAgentGone", err)
	}
	// A later call fails at once, and Stats report the death.
	callGame(a, context.Background(), nil, "u3.0", errCh)
	if err := <-errCh; !errors.Is(err, ErrAgentGone) {
		t.Fatalf("later call error %v, want ErrAgentGone", err)
	}
	if !a.Stats().Dead {
		t.Error("Stats().Dead = false after the agent died")
	}
	// The death is only noticed during the second call, so only the
	// first call's counters are pinned.
	if s := a.Stats(); s.Decides < 1 || s.Batches < 1 {
		t.Errorf("Stats = %+v, want at least 1 decide in 1 batch", s)
	}
}

// TestProtocolViolationKillsSession: a fake agent answers a batch with an
// unknown id — a violation that kills the session, wrapping both
// ErrProtocol and ErrAgentGone in the caller's error.
func TestProtocolViolationKillsSession(t *testing.T) {
	toAgentR, toAgentW := io.Pipe()
	fromAgentR, fromAgentW := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = toAgentR.Close(); _ = fromAgentW.Close() }()
		wr := wire.NewWriter(fromAgentW)
		rd := wire.NewReader(toAgentR, 0)
		hello, err := rd.Read()
		if err != nil {
			return
		}
		if err := hello.Validate(wire.FromGorge); err != nil || hello.Type != wire.THello {
			return
		}
		if err := wr.Write(validAgentHello("fake")); err != nil {
			return
		}
		batch, err := rd.Read()
		if err != nil || batch.Type != wire.TBatch {
			return
		}
		_ = wr.Write(wire.Message{V: wire.Version, Type: wire.TResult, Batch: batch.Batch,
			Answers: []wire.Answer{{ID: 999, Pick: "not-an-item"}}})
	}()
	a, err := DialConn(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, Options{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	errCh := make(chan error, 1)
	callGame(a, context.Background(), nil, "u1.0", errCh)
	err = <-errCh
	<-done
	_ = a.Close()
	if !errors.Is(err, ErrAgentGone) || !errors.Is(err, ErrProtocol) {
		t.Fatalf("call error %v, want both ErrAgentGone and ErrProtocol", err)
	}
	if !a.Stats().Dead {
		t.Error("Stats().Dead = false after a protocol violation")
	}
}
