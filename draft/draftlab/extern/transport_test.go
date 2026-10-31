package extern

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// TestExecTransport dials the test binary itself as the helper child
// (GORGE_EXTAGENT_HELPER=refagent), handshakes, answers one decide and
// closes cleanly.
func TestExecTransport(t *testing.T) {
	t.Setenv("GORGE_EXTAGENT_HELPER", "refagent")
	a, err := Dial(context.Background(), Endpoint{Transport: "exec", Target: os.Args[0]},
		Options{Stderr: io.Discard})
	if err != nil {
		t.Fatalf("dial exec: %v", err)
	}
	if a.Hello().Agent != "random" {
		t.Errorf("hello agent = %q, want %q", a.Hello().Agent, "random")
	}
	ans, err := a.call(context.Background(), nil, decideItem("u1.0"))
	if err != nil {
		t.Fatalf("decide call: %v", err)
	}
	if ans.Intent == nil || len(ans.Intent.Choices) != 1 || ans.Intent.Choices[0] != 0 {
		t.Fatalf("answer %+v, want intent {choices:[0]}", ans)
	}
	if s := a.Stats(); s.Decides != 1 || s.Batches != 1 {
		t.Errorf("Stats = %+v, want 1 decide in 1 batch", s)
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestUnixTransport serves the reference agent on a Unix socket and dials
// it with the unix transport.
func TestUnixTransport(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "a.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	served := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			served <- err
			return
		}
		defer c.Close()
		served <- refagent.Serve(c, c, refagent.Config{})
	}()
	a, err := Dial(context.Background(), Endpoint{Transport: "unix", Target: sock}, Options{})
	if err != nil {
		t.Fatalf("dial unix: %v", err)
	}
	if _, err := a.call(context.Background(), nil, decideItem("u1.0")); err != nil {
		t.Fatalf("decide call: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := <-served; err != nil {
		t.Errorf("serving agent: %v", err)
	}
}

// TestCloseSendsByeAndIsIdempotent: Close enqueues bye (last message the
// agent sees), waits the session out and returns the same result on a
// second call. The fake agent closes the pipes only after the bye, so
// Close's return proves the reader saw the stream end.
func TestCloseSendsByeAndIsIdempotent(t *testing.T) {
	toAgentR, toAgentW := io.Pipe()
	fromAgentR, fromAgentW := io.Pipe()
	var mu sync.Mutex
	var types []string
	sawBye := make(chan struct{})
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
		for {
			m, err := rd.Read()
			if err != nil {
				return
			}
			mu.Lock()
			types = append(types, m.Type)
			mu.Unlock()
			if m.Type == wire.TBye {
				close(sawBye)
				return
			}
		}
	}()
	a, err := DialConn(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, Options{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	select {
	case <-sawBye:
	default:
		t.Fatal("the agent never saw a bye")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(types) == 0 || types[len(types)-1] != wire.TBye {
		t.Errorf("messages seen %v, want bye last", types)
	}
	<-done
	// The writer goroutine must have exited after the bye: a stuck writer
	// (bye pending but never marked) hangs the process.
	<-a.sess.writerDoneCh
}

// TestDialContextCancelKillsChild: cancelling the Dial ctx while a call is
// pending kills the exec child (exec.CommandContext); the call fails with
// ErrAgentGone and Close returns once the child is reaped — either the
// call was already submitted (the silent child never answers, so it stays
// pending) or it failed at once on the dead session.
func TestDialContextCancelKillsChild(t *testing.T) {
	t.Setenv("GORGE_EXTAGENT_HELPER", "silent")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := Dial(ctx, Endpoint{Transport: "exec", Target: os.Args[0]}, Options{Stderr: io.Discard})
	if err != nil {
		t.Fatalf("dial exec: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := a.call(context.Background(), nil, decideItem("u1.0"))
		errCh <- err
	}()
	cancel()
	if err := <-errCh; !errors.Is(err, ErrAgentGone) {
		t.Fatalf("call error %v, want ErrAgentGone", err)
	}
	if err := a.Close(); err == nil {
		t.Error("Close returned nil after the child was killed by the Dial ctx; want the exit error")
	}
	if !a.Stats().Dead {
		t.Error("Stats().Dead = false after the Dial ctx was cancelled")
	}
}
