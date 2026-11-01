package extern

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/engine/view"
)

// FlushMode selects when a session flushes its outgoing queue.
type FlushMode string

const (
	// FlushBarrier (the default; "" means it) flushes when every live game
	// is waiting on an external agent, or a session's queue holds maxBatch
	// items.
	FlushBarrier FlushMode = "barrier"
	// FlushReady flushes whatever is queued whenever the session is idle
	// (inferbroker's rule).
	FlushReady FlushMode = "ready"
)

// Failure selects what a dead or misbehaving agent does to a request.
type Failure string

const (
	// FailError (the default; "" means it) fails every affected request with
	// an error wrapping ErrAgentGone.
	FailError Failure = "error"
	// FailFallback answers later requests with the built-in fallback instead
	// of failing them (counted in Stats.Fallbacks).
	FailFallback Failure = "fallback"
)

// Options configures one agent session.
type Options struct {
	// Flush is the flush mode; "" means FlushBarrier.
	Flush FlushMode
	// MaxBatch caps a batch's items. 0 = the hello's max_batch if > 0, else
	// 256; always capped at 1024.
	MaxBatch int
	// OnFailure selects the failure mode; "" means FailError. StrictRefusals
	// and OnFailure are consumed by the adapters; the session only carries
	// them.
	OnFailure Failure
	// StrictRefusals makes a refused or illegal answer an error (ErrRefused)
	// instead of a fallback.
	StrictRefusals bool
	// Log gets one line per session death and per end of session; nil is
	// silent.
	Log func(line string)
	// Stderr carries an exec child's stderr; nil means os.Stderr. Unused on
	// the unix transport.
	Stderr io.Writer
}

// Stats is one agent's counters, read atomically.
type Stats struct {
	// Picks, Builds, Decides count the items accepted (queued) per kind.
	Picks, Builds, Decides int64
	// Batches counts the batches composed and sent.
	Batches int64
	// MaxBatch is the largest batch sent.
	MaxBatch int
	// Forced, Refused and Fallbacks are the adapters' counters; the
	// session layer leaves them at zero.
	Forced, Refused, Fallbacks int64
	// Dead reports a session that died (an agent error, a protocol
	// violation, EOF, a write failure or the Dial context ending).
	Dead bool
}

// Agent is one external agent session: a gorge-agent/1 peer dialed once
// (over a child's stdio or a Unix socket) that many concurrently running
// games share, batched by the process-wide hub. The adapters wrap one
// in a Drafter, Builder and Player.
type Agent struct {
	opts     Options
	hello    wire.Message
	omit     view.Omit
	maxBatch int
	hub      *hub
	sess     *session

	statsPicks, statsBuilds, statsDecides atomic.Int64
	statsBatches, statsMaxBatch           atomic.Int64
	statsForced, statsRefused             atomic.Int64
	statsFallbacks                        atomic.Int64

	closeOnce sync.Once
	closeErr  error
}

// Hello is the agent's hello message from the handshake.
func (a *Agent) Hello() wire.Message { return a.hello }

// Ref names the agent for draftlab's version table: "ext:<hello.agent>",
// versioned with the hello's model_version. The transport target (a path
// or socket) is never recorded.
func (a *Agent) Ref() draftlab.AgentRef {
	return draftlab.AgentRef{Name: "ext:" + a.hello.Agent, Version: a.hello.ModelVersion}
}

// Stats reads the agent's counters.
func (a *Agent) Stats() Stats {
	return Stats{
		Picks:     a.statsPicks.Load(),
		Builds:    a.statsBuilds.Load(),
		Decides:   a.statsDecides.Load(),
		Batches:   a.statsBatches.Load(),
		MaxBatch:  int(a.statsMaxBatch.Load()),
		Dead:      a.sess.deadFlag.Load(),
		Forced:    a.statsForced.Load(),
		Refused:   a.statsRefused.Load(),
		Fallbacks: a.statsFallbacks.Load(),
	}
}

// effectiveMaxBatch resolves Options.MaxBatch, the hello's max_batch, then
// the 256 default, capped at 1024.
func effectiveMaxBatch(o Options, hello wire.Message) int {
	mb := o.MaxBatch
	if mb <= 0 {
		mb = hello.MaxBatch
	}
	if mb <= 0 {
		mb = 256
	}
	return min(mb, 1024)
}

// Dial opens the Endpoint's transport and handshakes. On exec it starts
// the child with exec.CommandContext(ctx, ...), so cancelling ctx kills
// the child and ends the session; the child's stderr goes to
// Options.Stderr (or os.Stderr). On unix it dials the socket with a plain
// net.Dialer. The Dial ctx bounds the whole session: when it ends, the
// session dies and every request fails with ErrAgentGone. There is no
// other deadline and no clock here; a hung agent hangs its
// games, and a training program wanting a timeout builds its own ctx.
func Dial(ctx context.Context, ep Endpoint, o Options) (*Agent, error) {
	switch ep.Transport {
	case "exec":
		target := ep.Target
		if !strings.ContainsRune(target, '/') {
			path, err := exec.LookPath(target)
			if err != nil {
				return nil, fmt.Errorf("%w: exec: %q: %v", ErrSpec, ep.Target, err)
			}
			target = path
		}
		cmd := exec.CommandContext(ctx, target)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("extern: exec: stdin pipe: %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			_ = stdin.Close()
			return nil, fmt.Errorf("extern: exec: stdout pipe: %v", err)
		}
		cmd.Stderr = o.Stderr
		if cmd.Stderr == nil {
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Start(); err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			return nil, fmt.Errorf("extern: exec: start %q: %v", ep.Target, err)
		}
		conn := &execConn{cmd: cmd, stdin: stdin, stdout: stdout}
		return dial(ctx, conn, o, procHub, conn.finish)
	case "unix":
		c, err := (&net.Dialer{}).DialContext(ctx, "unix", ep.Target)
		if err != nil {
			return nil, fmt.Errorf("extern: unix: dial %q: %v", ep.Target, err)
		}
		return dial(ctx, c, o, procHub, nil)
	default:
		return nil, fmt.Errorf("%w: transport %q is not exec or unix", ErrSpec, ep.Transport)
	}
}

// DialConn handshakes over a caller's connection — an io.Pipe pair in
// tests, or an in-process embedding — and binds the agent to the
// process-wide hub.
func DialConn(ctx context.Context, conn io.ReadWriteCloser, o Options) (*Agent, error) {
	return dial(ctx, conn, o, procHub, nil)
}

// dial is Dial/DialConn's shared body: handshake, then start the session
// goroutines on hub h. finish runs after the reader exits (exec's
// cmd.Wait). The handshake is read from the session's own *wire.Reader:
// a Reader owns a bufio.Reader, so reading the hello through a throwaway
// reader would silently discard agent bytes that followed it in one
// coalesced write.
func dial(ctx context.Context, conn io.ReadWriteCloser, o Options, h *hub, finish func() error) (*Agent, error) {
	wr := wire.NewWriter(conn)
	if err := wr.Write(gorgeHello()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: write: %v", ErrHandshake, err)
	}
	s := &session{
		wr:           wr,
		rd:           wire.NewReader(conn, 0),
		conn:         conn,
		pending:      map[uint64]*reqRec{},
		doneCh:       make(chan struct{}),
		writerDoneCh: make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	s.tr = transportFor(conn)

	m, err := s.rd.Read()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: read: %v", ErrHandshake, err)
	}
	if err := m.Validate(wire.FromAgent); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: %v", ErrHandshake, err)
	}
	if m.Type != wire.THello {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: field %q: got %q, want %q", ErrHandshake, "type", m.Type, wire.THello)
	}
	omit, err := wire.ParseViewOmit(m.ViewOmit)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: %v", ErrHandshake, err)
	}

	a := &Agent{
		opts:     o,
		hello:    m,
		omit:     omit,
		maxBatch: effectiveMaxBatch(o, m),
		hub:      h,
	}
	s.agent = a
	a.sess = s

	h.register(s)
	go s.writeLoop()
	go s.readLoop()
	if done := ctx.Done(); done != nil {
		go func() {
			select {
			case <-done:
				s.kill(gone(fmt.Sprintf("dial context cancelled: %v", context.Cause(ctx)), nil))
			case <-s.doneCh:
			}
		}()
	}
	return a, nil
}

// transportFor wires the half-close hooks for one conn kind.
func transportFor(conn io.ReadWriteCloser) transport {
	switch c := conn.(type) {
	case *execConn:
		return transport{writeCloser: c.stdin, closer: c.stdout, finish: c.finish}
	case interface{ CloseWrite() error }:
		return transport{writeCloser: closeWriteCloser{c}, closer: conn}
	}
	return transport{closer: conn}
}

// closeWriteCloser adapts a CloseWrite method to io.Closer.
type closeWriteCloser struct {
	c interface{ CloseWrite() error }
}

func (w closeWriteCloser) Close() error { return w.c.CloseWrite() }

// execConn is a started child's stdio pair as one io.ReadWriteCloser.
type execConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (c *execConn) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *execConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }

// Close closes both pipe halves. It runs after the reader goroutine has
// exited, so it cannot race the session's reads.
func (c *execConn) Close() error {
	_ = c.stdin.Close()
	return c.stdout.Close()
}

// finish reaps the child; it is safe only after the reader goroutine has
// exited (a stdout pipe must not be read during Wait).
func (c *execConn) finish() error { return c.cmd.Wait() }

// gorgeHello is the hello gorge writes first: the protocol, this build's
// revision (informational) and the item kinds gorge may ask.
func gorgeHello() wire.Message {
	return wire.Message{
		V:             wire.Version,
		Type:          wire.THello,
		Protocol:      wire.Protocol,
		GorgeRevision: gorgeRevision(),
		Kinds:         []string{wire.KindPick, wire.KindBuild, wire.KindDecide},
	}
}

// gorgeRevision is the build's vcs.revision ("+dirty" when the build
// carried uncommitted changes), else the module version, else "devel".
// It is computed once. (The same shape draftlab's manifest carries.)
var gorgeRevision = sync.OnceValue(func() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		rev, dirty := "", false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" {
			if dirty {
				rev += "+dirty"
			}
			return rev
		}
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "devel"
})

// Close shuts the session down: it enqueues bye, closes the write half
// (the child's stdin, or the socket's) once the queue drains, waits for
// the reader to see the stream end, then reaps an exec child. It is
// idempotent and returns the first real error; an exit after bye is not
// an error. A child that ignores bye holds Close until the Dial context
// is cancelled (which kills it); cancelling the Dial ctx is the only
// escape — there is no clock here.
func (a *Agent) Close() error {
	a.closeOnce.Do(func() {
		s := a.sess
		s.mu.Lock()
		if !s.deadFlag.Load() && !s.closing {
			s.closing = true
			s.byePending = true
			s.outQ = append(s.outQ, wire.Message{V: wire.Version, Type: wire.TBye})
			s.cond.Broadcast()
		}
		s.mu.Unlock()
		<-s.doneCh
		var err error
		if s.tr.finish != nil {
			err = s.tr.finish()
		}
		_ = s.conn.Close()
		a.closeErr = err
	})
	return a.closeErr
}
