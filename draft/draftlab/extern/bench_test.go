package extern

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// BenchmarkGames measures what the external-agent path costs against the
// built-in bot, in games per hour, for the configurations the operator must
// choose between (plan §11, §15 Q1). Every sub-benchmark plays b.N games
// through ONE Lab.Games call: the first two fdn-limited catalog decks, the
// opponent always bot@1, and the measured player alternating sides. Games
// run under a 40-turn / 4000-intent watchdog so a random agent that wanders
// into the payment path stalls a bounded game rather than hanging the run;
// the same cap applies to every row, so the external/builtin ratio is the
// honest comparison.
//
// Run it (it is not a merge gate):
//
//	systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% \
//	  env GOMAXPROCS=4 GOMEMLIMIT=3GiB \
//	  go test -timeout 2m -run '^$' -bench 'BenchmarkGames' -benchtime 40x ./draft/draftlab/extern
func BenchmarkGames(b *testing.B) {
	cases := []benchCase{
		{name: "builtin-bot", workers: 4},
		{name: "ext-pipe-lean", external: true, transport: "pipe", lean: true, flush: FlushBarrier, workers: 4},
		{name: "ext-stdio-lean", external: true, transport: "stdio", lean: true, flush: FlushBarrier, workers: 4},
		{name: "ext-stdio-full", external: true, transport: "stdio", lean: false, flush: FlushBarrier, workers: 4},
		{name: "ext-stdio-lean-ready", external: true, transport: "stdio", lean: true, flush: FlushReady, workers: 4},
		{name: "ext-unix-lean", external: true, transport: "unix", lean: true, flush: FlushBarrier, workers: 4},
		{name: "ext-stdio-lean-w16", external: true, transport: "stdio", lean: true, flush: FlushBarrier, workers: 16},
	}
	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) { runBenchCase(b, c) })
	}
}

// benchCase is one sub-benchmark's configuration: the built-in bot, or an
// external refagent over one transport, with the lean (every view_omit) or
// full projection, one flush rule and one worker count.
type benchCase struct {
	name      string
	external  bool
	transport string // "pipe", "stdio" or "unix"
	lean      bool
	flush     FlushMode
	workers   int
}

// runBenchCase plays b.N games for one configuration and reports games/h,
// decisions/game, batch/mean and stall/frac. The dial, the lab open and the
// matchup build are all outside the timer; the measured region is the one
// Games call.
func runBenchCase(b *testing.B, c benchCase) {
	b.Helper()
	l, err := draftlab.Open(testutil.CorpusDirOrSkip(b), draftlab.Options{
		Workers:    c.workers,
		MaxTurns:   40,
		MaxIntents: 4000,
	})
	if err != nil {
		b.Fatalf("draftlab Open: %v", err)
	}
	decks, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		b.Fatalf("CatalogDecks: %v", err)
	}
	if len(decks) < 2 {
		b.Fatalf("fdn-limited catalog has %d decks, want at least 2", len(decks))
	}
	bot, err := l.Player(draftlab.AgentRef{Name: "bot", Version: "1"})
	if err != nil {
		b.Fatalf("Player(bot): %v", err)
	}

	var agent *Agent
	measured := bot
	if c.external {
		agent = dialBench(b, c)
		measured, err = agent.Player(l)
		if err != nil {
			b.Fatalf("Player(ext): %v", err)
		}
	}

	ms := make([]draftlab.Matchup, b.N)
	for i := range ms {
		a, bb := draftlab.Entrant{Deck: decks[0], Player: bot}, draftlab.Entrant{Deck: decks[1], Player: measured}
		if i%2 == 0 {
			a, bb = draftlab.Entrant{Deck: decks[0], Player: measured}, draftlab.Entrant{Deck: decks[1], Player: bot}
		}
		ms[i] = draftlab.Matchup{A: a, B: bb}
	}

	b.ResetTimer()
	res, err := l.Games(ms, 1)
	b.StopTimer()
	if agent != nil {
		_ = agent.Close()
	}
	if err != nil {
		b.Fatalf("Games: %v", err)
	}

	elapsed := b.Elapsed()
	b.ReportMetric(3.6e12*float64(b.N)/float64(elapsed.Nanoseconds()), "games/h")

	var decPerGame, batchMean float64
	if agent != nil {
		st := agent.Stats()
		decPerGame = float64(st.Decides) / float64(b.N)
		if st.Batches > 0 {
			batchMean = float64(st.Decides) / float64(st.Batches)
		}
	} else {
		total := 0
		for _, r := range res {
			total += r.Intents
		}
		decPerGame = float64(total) / float64(2*b.N)
	}
	b.ReportMetric(decPerGame, "decisions/game")
	b.ReportMetric(batchMean, "batch/mean")

	stalled := 0
	for _, r := range res {
		if r.Stall != "" {
			stalled++
		}
	}
	b.ReportMetric(float64(stalled)/float64(b.N), "stall/frac")
}

// dialBench opens the external agent for one sub-benchmark: an in-process
// refagent over io.Pipe (DialConn), an exec child of this test binary (the
// GORGE_EXTAGENT_HELPER pattern), or a refagent served on a Unix socket by a
// goroutine in the test. Every agent is closed after the measured Games
// call; the cleanup is idempotent.
func dialBench(b *testing.B, c benchCase) *Agent {
	b.Helper()
	cfg := refagent.Config{Policy: "random"}
	if c.lean {
		cfg.ViewOmit = benchOmitNames()
	}
	o := Options{Flush: c.flush, Stderr: io.Discard}
	switch c.transport {
	case "pipe":
		return dialBenchPipe(b, cfg, o)
	case "unix":
		return dialBenchUnix(b, cfg, o)
	default:
		return dialBenchStdio(b, cfg, o)
	}
}

// benchOmitNames is every view.Omit name (wire.ViewOmitNames), the lean set.
func benchOmitNames() []string {
	names := make([]string, 0, len(wire.ViewOmitNames))
	for _, e := range wire.ViewOmitNames {
		names = append(names, e.Name)
	}
	return names
}

// dialBenchStdio re-executes this test binary as the agent child: TestMain
// sees GORGE_EXTAGENT_HELPER and serves a refagent (main_test.go).
func dialBenchStdio(b *testing.B, cfg refagent.Config, o Options) *Agent {
	b.Helper()
	raw, err := json.Marshal(helperConfig{Policy: cfg.Policy, ViewOmit: cfg.ViewOmit})
	if err != nil {
		b.Fatalf("marshal helper config: %v", err)
	}
	b.Setenv("GORGE_EXTAGENT_HELPER", string(raw))
	a, err := Dial(context.Background(), Endpoint{Transport: "exec", Target: os.Args[0]}, o)
	if err != nil {
		b.Fatalf("dial stdio: %v", err)
	}
	return a
}

// dialBenchPipe serves a refagent in-process over io.Pipe pairs and dials it
// with DialConn -- the protocol cost without a process.
func dialBenchPipe(b *testing.B, cfg refagent.Config, o Options) *Agent {
	b.Helper()
	toAgentR, toAgentW := io.Pipe()
	fromAgentR, fromAgentW := io.Pipe()
	served := make(chan error, 1)
	go func() {
		err := refagent.Serve(toAgentR, fromAgentW, cfg)
		_ = toAgentR.Close()
		_ = fromAgentW.Close()
		served <- err
	}()
	a, err := DialConn(context.Background(), pipeConn{Reader: fromAgentR, WriteCloser: toAgentW}, o)
	if err != nil {
		b.Fatalf("dial pipe: %v", err)
	}
	b.Cleanup(func() {
		_ = a.Close()
		if err := <-served; err != nil && err != io.EOF {
			b.Logf("refagent serve: %v", err)
		}
	})
	return a
}

// dialBenchUnix serves a refagent on a Unix socket in a goroutine and dials
// it with the unix transport.
func dialBenchUnix(b *testing.B, cfg refagent.Config, o Options) *Agent {
	b.Helper()
	sock := filepath.Join(b.TempDir(), "a.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		b.Fatalf("listen unix: %v", err)
	}
	served := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			served <- err
			return
		}
		defer c.Close()
		served <- refagent.Serve(c, c, cfg)
	}()
	a, err := Dial(context.Background(), Endpoint{Transport: "unix", Target: sock}, o)
	if err != nil {
		_ = ln.Close()
		b.Fatalf("dial unix: %v", err)
	}
	b.Cleanup(func() {
		_ = a.Close()
		_ = ln.Close()
		if err := <-served; err != nil && err != io.EOF {
			b.Logf("serving agent: %v", err)
		}
	})
	return a
}
