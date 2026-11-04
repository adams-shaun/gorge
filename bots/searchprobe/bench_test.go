package searchprobe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// benchRoot is the shared end-to-end fixture: the approved pair the teacher's
// cost was measured on (mono-red-prowess vs mono-blue-tempo), played bot vs
// bot to seat 0's first priority decision on or after benchRootTurn, with the
// observation history a driver would have collected on the way. It is the
// realistic shape of one searched decision: a few turns of frames, real
// corpus cards, opponent hidden zones the sampler must propose.
const benchRootTurn = 5

type benchFixture struct {
	setup     PublicGame
	h         History
	engine    *rules.Engine
	collector *Collector
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
			return benchFixture{setup: setup, h: h, engine: e, collector: c}
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
	// the search teacher's defaults: the knobs the 521 ms figure was taken at.
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
// to be justified as such, not as an optimisation. An ENGINE behaviour change
// that moves a captured frame's content moves it too, legitimately: the frames
// seed the sampler, so a changed ask label inside one moves the frame-seeded
// worlds even when the fixture game itself does not diverge.
func TestSampleRealDeckGolden(t *testing.T) {
	f := benchRoot(t)
	for _, tc := range []struct {
		name            string
		noLandExclusion bool
		want            string
	}{
		// The digest with the one distribution-preserving proposal change switched
		// off: the sampler must still draw byte-identical worlds. A view field added
		// to the captured frames moves it, because the frames' bytes seed the worlds;
		// attribute such a move by hiding the field (json:"-"), which must restore the
		// old digest. Measured: frames 126, attempts 64, accepted 13, worlds 8.
		{"pre-optimisation sampler", true, "0a8a0ed57f81d1c54d9e0b8adc5113e54520b79497726de5171e4e313bd29f49"},
		// With the declined-land-drop exclusion: different proposals (so
		// different worlds for a seed), same target distribution -- see
		// TestLandExclusionRemovesOnlyRejectedWorlds. Measured: accepted 31, ESS 31.
		{"land exclusion", false, "be1b093615ab21404ab4bfec17bf735377e55cf62642bc612e017084ac5be487"},
	} {
		opts := benchSampleOptions()
		opts.MinESS = 1 // resample worlds from the thin pool so the digest covers them
		opts.NoLandExclusion = tc.noLandExclusion
		res, err := Sample(f.setup, f.h, opts)
		if err != nil {
			t.Fatal(err)
		}
		if res.Accepted == 0 || len(res.Worlds) == 0 {
			t.Fatalf("%s: fixture accepts no world: %+v", tc.name, res)
		}
		if got := worldsDigest(t, res); got != tc.want {
			t.Errorf("%s: sampler output moved: digest %s, want %s (frames %d attempts %d accepted %d worlds %d ESS %.3f)",
				tc.name, got, tc.want, len(f.h.Frames), res.Attempts, res.Accepted, len(res.Worlds), res.ESS)
		}
		// Parallelism is wall clock only. (Checked on the live proposal; the
		// teacher golden covers the rollout side.)
		if tc.noLandExclusion {
			continue
		}
		opts.Parallelism = 4
		par, err := Sample(f.setup, f.h, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := worldsDigest(t, par); got != tc.want {
			t.Errorf("%s: Parallelism 4 changed the sampler's output: %s", tc.name, got)
		}
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
	// The pre-optimisation proposal, so the teacher golden below is comparable
	// with the digest taken before the performance work.
	opts.NoLandExclusion = true
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
	// Only the worlds move this digest legitimately: the captured frames seed
	// them, so a change to a frame's bytes (a view field, an ask label, a priority
	// window) makes the rollouts run on different games and moves Submits with no
	// rollout-side change. Attribute such a move by hiding the field (json:"-") or
	// forcing the behaviour off, which must restore the old digest. The hashed JSON
	// includes WinsOverBaseline/LossesToBaseline, the paired terminal outcomes
	// against candidate 0. Measured: Rollouts 32, Submits 5458, Terminal 32,
	// Capped 0.
	const want = "c6078ba0d54ec8d3fdc094dee87ed1fe3453c31e32754868de748484c4e83647"
	for _, parallelism := range []int{0, 4} {
		res, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 99, MaxSubmits: 5000, Parallelism: parallelism})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(res)
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("Parallelism %d: teacher output moved: digest %s, want %s: %s", parallelism, got, want, raw)
		}
	}
}

func BenchmarkSampleRealDecksParallel4(b *testing.B) {
	f := benchRoot(b)
	opts := benchSampleOptions()
	opts.Parallelism = 4
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Sample(f.setup, f.h, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTeacherChoiceRealDecksParallel4(b *testing.B) {
	worlds, cands := benchTeacherInputs(b)
	opts := TeacherOptions{Seed: 99, MaxSubmits: 5000, Parallelism: 4}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := TeacherChoice(worlds, cands, opts); err != nil {
			b.Fatal(err)
		}
	}
}
