package searchprobe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
)

// benchRoot is the shared end-to-end fixture: the approved pair the teacher's
// cost was measured on (mono-red-prowess vs mono-blue-tempo), played bot vs
// bot to seat 0's first priority decision on or after benchRootTurn, with the
// observation history a driver would have collected on the way. It is the
// realistic shape of one searched decision: a few turns of frames, real
// corpus cards, opponent hidden zones the sampler must propose.
const benchRootTurn = 5

type benchFixture struct {
	setup  PublicGame
	h      History
	engine *rules.Engine
}

func benchRoot(tb testing.TB) benchFixture {
	tb.Helper()
	reg := testutil.CorpusRegistry(tb)
	names := []string{"mono-red-prowess", "mono-blue-tempo"}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		var err error
		if decks[i], err = testutil.LoadRepoDeck(reg, n); err != nil {
			tb.Fatal(err)
		}
	}
	setup := PublicGame{Names: names, Decks: decks, Tokens: reg.Tokens}
	e := rules.New(rules.Config{Seed: 30_000_000, Names: names, Decks: decks, Tokens: reg.Tokens})
	e.Advance()
	c := NewCollector(0)
	h := History{Actor: 0, Answers: make(map[int][]Action)}
	rngs := BotRandoms(7, len(names))
	board := botpolicy.NewBoard(len(names))
	pos := 0
	for i := 0; i < 2000; i++ {
		frame, err := c.Capture(e, e.L.Events[pos:])
		if err != nil {
			tb.Fatal(err)
		}
		h.Frames = append(h.Frames, frame)
		d := e.Pending()
		if d == nil {
			tb.Fatal("bench fixture game ended before its root")
		}
		if d.Player == 0 && d.Kind == decision.KPriority && e.G.Turn >= benchRootTurn && len(benchCandidates(frame.Decision)) >= 2 {
			return benchFixture{setup: setup, h: h, engine: e}
		}
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if d.Player == 0 {
			if h.Answers[i], err = c.Actions(d, in); err != nil {
				tb.Fatal(err)
			}
		}
		pos = len(e.L.Events)
		if err := e.Submit(in); err != nil {
			tb.Fatal(err)
		}
	}
	tb.Fatal("bench fixture never reached its root")
	return benchFixture{}
}

func benchSampleOptions() SampleOptions {
	// cmd/searchteacher's defaults: the knobs the 521 ms figure was taken at.
	return SampleOptions{Seed: 54321, Attempts: 64, Worlds: 8, MaxSubmits: 5000}
}

// worldsDigest is a stable fingerprint of a SampleResult's observable output:
// the counters plus every sampled world's chain head and RNG position. Two
// Sample implementations that draw the same worlds agree on it byte for byte.
func worldsDigest(tb testing.TB, r SampleResult) string {
	tb.Helper()
	type world struct {
		Head  string
		Draws uint64
		N     int
	}
	var ws []world
	for _, w := range r.Worlds {
		ws = append(ws, world{Head: w.Engine.L.Head(), Draws: w.Engine.RNGDraws(), N: len(w.Engine.L.Events)})
	}
	raw, err := json.Marshal(struct {
		R SampleResult
		W []world
	}{r, ws})
	if err != nil {
		tb.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// TestSampleRealDeckGolden pins the sampler's exact output on the bench
// fixture. A performance change to Sample, Capture or the proposal machinery
// must not move it: same seed, same worlds, same counters. If it moves, the
// accepted-world distribution (or its diagnostics) changed and the change has
// to be justified as such, not as an optimisation.
func TestSampleRealDeckGolden(t *testing.T) {
	f := benchRoot(t)
	opts := benchSampleOptions()
	opts.MinESS = 1 // resample worlds from the thin pool so the digest covers them
	res, err := Sample(f.setup, f.h, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted == 0 || len(res.Worlds) == 0 {
		t.Fatalf("fixture accepts no world: %+v", res)
	}
	const want = "91def6c77533be19728e00036cb9373dab3eb53a4424bc38ae13e5d0c89655bd"
	if got := worldsDigest(t, res); got != want {
		t.Fatalf("sampler output moved: digest %s, want %s (frames %d attempts %d accepted %d worlds %d ESS %.3f)",
			got, want, len(f.h.Frames), res.Attempts, res.Accepted, len(res.Worlds), res.ESS)
	}
}

func BenchmarkSampleRealDecks(b *testing.B) {
	f := benchRoot(b)
	opts := benchSampleOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Sample(f.setup, f.h, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSampleAttemptRealDecks is one proposal attempt (Attempts 1): the
// unit the sampler's cost is a multiple of.
func BenchmarkSampleAttemptRealDecks(b *testing.B) {
	f := benchRoot(b)
	opts := benchSampleOptions()
	opts.Attempts, opts.Worlds = 1, 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Sample(f.setup, f.h, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCaptureRealDecks is Capture on a developed real-deck board (the
// per-frame cost of the replay), unlike BenchmarkCollectorCaptureRepeated's
// synthetic opening position.
func BenchmarkCaptureRealDecks(b *testing.B) {
	f := benchRoot(b)
	c := NewCollector(0)
	if _, err := c.Capture(f.engine, f.engine.L.Events); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Capture(f.engine, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func benchTeacherInputs(tb testing.TB) ([]World, [][]Action) {
	tb.Helper()
	f := benchRoot(tb)
	opts := benchSampleOptions()
	opts.MinESS = 1
	res, err := Sample(f.setup, f.h, opts)
	if err != nil {
		tb.Fatal(err)
	}
	if len(res.Worlds) == 0 {
		tb.Fatalf("bench fixture sampled no worlds: %+v", res)
	}
	return res.Worlds, benchCandidates(f.h.Frames[len(f.h.Frames)-1].Decision)
}

// benchCandidates is the root's pass plus up to three casts, in offered order.
func benchCandidates(root *ObservedDecision) [][]Action {
	var cands [][]Action
	if root == nil {
		return nil
	}
	for _, o := range root.Options {
		if len(cands) < 4 && (o.Action.Kind == "pass" || o.Action.Kind == "cast") {
			cands = append(cands, []Action{o.Action})
		}
	}
	return cands
}

// BenchmarkTeacherChoiceRealDecks rolls every candidate on every sampled world
// to game end: the search half of a searched decision.
func BenchmarkTeacherChoiceRealDecks(b *testing.B) {
	worlds, cands := benchTeacherInputs(b)
	opts := TeacherOptions{Seed: 99, MaxSubmits: 5000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := TeacherChoice(worlds, cands, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// TestTeacherChoiceRealDeckGolden pins the rollout values on the bench
// fixture: a rollout-side optimisation must not move any candidate's value.
func TestTeacherChoiceRealDeckGolden(t *testing.T) {
	worlds, cands := benchTeacherInputs(t)
	res, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 99, MaxSubmits: 5000})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	sum := sha256.Sum256(raw)
	const want = "71d2a8f07a7532c0a7b867d871b0c54ba424a429701600e13382c776fe081ddf"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("teacher output moved: digest %s, want %s: %s", got, want, raw)
	}
}
