package extern

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// TestMain re-executes this test binary as the external agent child when
// GORGE_EXTAGENT_HELPER is set (no helper binary is ever built). The value
// is either a bare mode name — "refagent" serves the reference agent over
// stdio, "silent" handshakes and then never answers (a child the Dial ctx
// must kill) — or a JSON object configuring the refagent helper with a
// refagent.Config subset: mode, policy, model_version, view_omit,
// die_after, skip_forced, payment_actions, target_hints, kinds, max_batch,
// agent.
func TestMain(m *testing.M) {
	v := os.Getenv("GORGE_EXTAGENT_HELPER")
	switch v {
	case "":
		os.Exit(m.Run())
	case "refagent":
		_ = refagent.Serve(os.Stdin, os.Stdout, refagent.Config{})
	case "silent":
		silentServe(os.Stdin, os.Stdout)
	default:
		var c helperConfig
		if err := json.Unmarshal([]byte(v), &c); err != nil {
			fmt.Fprintf(os.Stderr, "GORGE_EXTAGENT_HELPER: not a mode or JSON config: %v\n", err)
			os.Exit(1)
		}
		if c.Mode == "silent" {
			silentServe(os.Stdin, os.Stdout)
			os.Exit(0)
		}
		_ = refagent.Serve(os.Stdin, os.Stdout, c.refagentConfig())
	}
	os.Exit(0)
}

// helperConfig is the JSON shape GORGE_EXTAGENT_HELPER accepts, a subset
// of refagent.Config.
type helperConfig struct {
	Mode           string   `json:"mode"` // "" = refagent; "silent" supported
	Policy         string   `json:"policy"`
	ModelVersion   string   `json:"model_version"`
	Agent          string   `json:"agent"`
	Kinds          []string `json:"kinds"`
	MaxBatch       int      `json:"max_batch"`
	ViewOmit       []string `json:"view_omit"`
	SkipForced     bool     `json:"skip_forced"`
	PaymentActions bool     `json:"payment_actions"`
	TargetHints    bool     `json:"target_hints"`
	DieAfter       int      `json:"die_after"`
}

// refagentConfig maps the helper config onto a served refagent.Config.
func (c helperConfig) refagentConfig() refagent.Config {
	return refagent.Config{
		Agent:          c.Agent,
		ModelVersion:   c.ModelVersion,
		Policy:         c.Policy,
		Kinds:          c.Kinds,
		MaxBatch:       c.MaxBatch,
		ViewOmit:       c.ViewOmit,
		SkipForced:     c.SkipForced,
		PaymentActions: c.PaymentActions,
		TargetHints:    c.TargetHints,
		DieAfter:       c.DieAfter,
	}
}

// silentServe handshakes like an agent and then reads and never answers:
// a hang only the Dial context can end.
func silentServe(r io.Reader, w io.Writer) {
	wr := wire.NewWriter(w)
	rd := wire.NewReader(r, 0)
	hello, err := rd.Read()
	if err != nil {
		return
	}
	if err := hello.Validate(wire.FromGorge); err != nil || hello.Type != wire.THello {
		return
	}
	_ = wr.Write(wire.Message{
		V: wire.Version, Type: wire.THello, Protocol: wire.Protocol,
		Agent: "silent", ModelVersion: "1", Kinds: []string{wire.KindDecide},
	})
	for {
		if _, err := rd.Read(); err != nil {
			return
		}
	}
}

// pipeConn joins a pipe's read half and another pipe's write half into one
// io.ReadWriteCloser for DialConn.
type pipeConn struct {
	io.Reader
	io.WriteCloser
}

func (p pipeConn) Close() error {
	err := p.WriteCloser.Close()
	if rc, ok := p.Reader.(io.Closer); ok {
		_ = rc.Close()
	}
	return err
}

// batchObs collects every batch item count the serving agent observed.
type batchObs struct {
	mu sync.Mutex
	ns []int
}

func (b *batchObs) record(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ns = append(b.ns, n)
}

func (b *batchObs) got() []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]int(nil), b.ns...)
}

func (b *batchObs) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.ns)
}

// dialRefagent serves a refagent over in-process pipes on hub h and dials
// it. Both goroutines and the agent are cleaned up with the test.
func dialRefagent(t *testing.T, h *hub, cfg refagent.Config, o Options) *Agent {
	t.Helper()
	toAgentR, toAgentW := io.Pipe()     // the agent reads; we write
	fromAgentR, fromAgentW := io.Pipe() // the agent writes; we read
	served := make(chan error, 1)
	go func() {
		err := refagent.Serve(toAgentR, fromAgentW, cfg)
		_ = toAgentR.Close()
		_ = fromAgentW.Close()
		served <- err
	}()
	t.Cleanup(func() {
		if err := <-served; err != nil && err != io.EOF {
			t.Logf("refagent serve: %v", err)
		}
	})
	a, err := dial(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, o, h, nil)
	if err != nil {
		t.Fatalf("dial refagent: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// decideItem is a minimal valid decide request: one forced-shape priority
// decision whose only option is pass, so the refagent's answer is
// intent {choices: [0]}.
func decideItem(stream string) wire.Item {
	return wire.Item{
		Kind:   wire.KindDecide,
		Seed:   7,
		Stream: stream,
		View:   json.RawMessage(`{"decision":{"kind":"priority","min":1,"max":1,"options":[{"index":0,"kind":"pass"}]}}`),
	}
}

// callGame runs one call on game key gk and reports its error on ch.
func callGame(a *Agent, ctx context.Context, gk any, stream string, ch chan<- error) {
	_, err := a.call(ctx, gk, decideItem(stream))
	ch <- err
}

// wantAnswer fails the test when a call's error is not nil.
func wantAnswer(t *testing.T, errCh chan error, where string) {
	t.Helper()
	if err := <-errCh; err != nil {
		t.Fatalf("%s: call failed: %v", where, err)
	}
}

// assertObs checks the observed batch item counts against want exactly.
func assertObs(t *testing.T, b *batchObs, want []int, where string) {
	t.Helper()
	got := b.got()
	if len(got) != len(want) {
		t.Fatalf("%s: observed batches %v, want %v", where, got, want)
	}
	seen := map[int]int{}
	for _, n := range got {
		seen[n]++
	}
	for _, n := range want {
		if seen[n] == 0 {
			t.Fatalf("%s: observed batches %v, want the multiset %v", where, got, want)
		}
		seen[n]--
	}
}

// validAgentHello is a well-formed agent hello a fake test agent sends.
func validAgentHello(agent string) wire.Message {
	return wire.Message{
		V: wire.Version, Type: wire.THello, Protocol: wire.Protocol,
		Agent: agent, ModelVersion: "1", Kinds: []string{wire.KindDecide},
	}
}
