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
// that moves a captured frame's content moves it too, legitimately: the
// digests were re-measured by cli-20260922T150843Z-c6c925c4 (Counter
// UnlessCost$ X/SVar fold), whose one measured divergence on this fixture is
// event index 305 — a mode_chosen unless-pay ask label inside the captured
// frame (mono-blue-tempo's Mausoleum Wanderer), "Pay the cost — don't
// counter" -> "Pay {1} — don't counter". The fixture game itself does not
// diverge (same event count, same root at iter 121), so the sampler's
// accepted-world count is unchanged; only the frame-seeded worlds moved.
func TestSampleRealDeckGolden(t *testing.T) {
	f := benchRoot(t)
	for _, tc := range []struct {
		name            string
		noLandExclusion bool
		want            string
	}{
		// The digest taken at 0b6e568b, before any of the performance work:
		// with the one distribution-preserving proposal change switched off,
		// the sampler still draws byte-identical worlds. Re-measured for the
		// Mausoleum Wanderer unless-cost ask label (see the test comment).
		// Re-measured again for the unless-pay mana window
		// (cli-20260922T150843Z-daf1bd3e): the same ask is now decline-only,
		// because the offer gate proves that payer cannot reach the {1}.
		// Neutralising that change's two switches (poseUnlessAsk's payability
		// consult and resumeResolution's guard plus window arm) reproduces
		// dfe3967e... and bfa2b184... byte-for-byte, so it is the sole mover
		// of both sampler digests and of the teacher digest below.
		// Re-measured for game-long damage-by-source provenance
		// (agent-20260923T114033Z-a57ee463): every landed Damage event now
		// emits a DamageProvenance fact, so the fixture game's captured frames
		// carry extra events and the sampled worlds' chain heads and event
		// counts move. Measured attribution: disabling ONLY the provenance
		// emission block in engine/rules/engine.go restores 857589... and 1dab03...
		// byte-for-byte, so it is the sole mover; frames/attempts/accepted/
		// worlds/ESS are unchanged.
		// CR 309 dungeon state adds completed_dungeons: 0 to each public
		// player view. The fixture's captured frames seed the sampler, so
		// their changed bytes move the digest; frames 122, attempts 64,
		// accepted 3 and worlds 8 remain unchanged.
		// Re-measured for the opponent archetype posterior
		// (agent-20260930T000950Z-25c1cb85): PlayerView now carries an
		// Archetype posterior for every non-viewer seat, so the fixture's
		// captured frames (which marshal the projected view) change bytes and
		// the sampled worlds move. frames 122, attempts 64, worlds 8 and the
		// accepted/ESS shape are unchanged.
		// Re-measured for CR 103.8a's turn-1 draw STEP skip (2026-10-06):
		// the fixture game loses the starting player's turn-1 draw-step
		// priority frames (122 -> 120 frames), so the captured history that
		// seeds the sampler moves. Measured after: attempts 64, accepted 4,
		// worlds 8, ESS 4 (land exclusion: accepted 27, ESS 27).
		// Re-measured for the per-turn watcher view fields (eng-turnwatch):
		// PlayerView gained spells_cast/life_gained/life_lost/tokens_created
		// _this_turn, so the captured frames marshal four more keys per seat
		// and the sampled worlds move. Attributed by hiding only those four
		// fields (json:"-"): the old digests return byte-for-byte. Measured
		// after: frames 120, attempts 64, accepted 12, worlds 8, ESS 12
		// (land exclusion: accepted 31, ESS 31).
		// Re-measured for the token-art printed-P/T field (token-art-pt-key):
		// CardView gained printed_pt, so the captured frames marshal one more
		// key per token and the sampled worlds move. Attributed by hiding only
		// that field (json:"-"): the old digests return byte-for-byte.
		// Measured after: frames 120, attempts 64, accepted 10, worlds 8,
		// ESS 10 (land exclusion: accepted 29, ESS 29).
		// Re-measured for the combat priority fixes (rules e7aaf1530 and
		// 93b6057e4, CR 510.4/117.3a): the fixture game poses more combat
		// priority windows before its root (120 -> 124 frames), so the
		// captured history that seeds the sampler moves. Not moved by the
		// view's derived type line (26ea2ed4f; same digests with ViewTypes
		// unwired). Measured after: frames 124, attempts 64, accepted 7,
		// worlds 8, ESS 7 (land exclusion: accepted 23, ESS 23).
		// Re-measured for the CR 510.3 post-damage priority round (rules
		// 24bbba791): 124 -> 126 frames. Measured after: attempts 64,
		// accepted 7, worlds 8, ESS 7 (land exclusion: accepted 31, ESS 31).
		// Re-measured for the A2 targeting port (view.CardView.ColourIdentity):
		// the captured frames marshal one more key per token, so the sampled
		// worlds move exactly as they did for printed_pt. Attributed by hiding
		// only that field (json:"-"): the old digests return byte-for-byte.
		// Measured after: frames 126, attempts 64, accepted 8, worlds 8,
		// ESS 8 (land exclusion: accepted 30, ESS 30).
		// Re-measured for the A1 removal-now port (view.CardView.RemovalKind,
		// RemovalTargets, RemovalDamage): the captured frames marshal three
		// more keys per card, so the sampled worlds move exactly as they did
		// for ColourIdentity. Attributed by hiding the three fields (json:"-"):
		// the old digests return byte-for-byte. Measured after: frames 126,
		// attempts 64, accepted 6, worlds 8, ESS 6 (land exclusion: accepted
		// 24, ESS 24).
		// Re-measured for the A3 Forge AI-hint port (view.CardView.PlayMain1,
		// RemoveDeck): the captured frames marshal two more keys for hinted
		// cards, so the sampled worlds move. Attributed by hiding the two
		// fields (json:"-"): the old digests return byte-for-byte. Measured
		// after: frames 126, attempts 64, accepted 10, worlds 8, ESS 10 (land
		// exclusion: accepted 22, ESS 22).
		// Re-measured for the A6a pump-timing port (view.CardView.PumpPower,
		// PumpToughness): the captured frames marshal two more keys per card,
		// so the sampled worlds move. Attributed by hiding the two fields
		// (json:"-"): the old digests return byte-for-byte. Measured after:
		// frames 126, attempts 64, accepted 6, worlds 8, ESS 6 (land
		// exclusion: accepted 20, ESS 20).
		// Re-measured for the A6a restricted-target-scope fact
		// (view.CardView.PumpOwn): the captured frames marshal one more key
		// per card, so the sampled worlds move again. Attributed by hiding
		// that field (json:"-"): the previous digests return byte-for-byte.
		// Measured after: frames 126, attempts 64, accepted 9, worlds 8,
		// ESS 9 (land exclusion: accepted 32, ESS 32).
		// Re-measured for cr-g67b (CR 602.2a): an activation's mint now
		// precedes its first decision, so the captured frames' stack moves.
		// Measured after: frames 126, attempts 64, accepted 4, worlds 8, ESS 4
		// (land exclusion: accepted 25, ESS 25).
		{"pre-optimisation sampler", true, "1e7df4d6ca1595dd000618c11501418d3e7879612ae105d2cde94b8134e1727b"},
		// With the declined-land-drop exclusion: different proposals (so
		// different worlds for a seed), same target distribution -- see
		// TestLandExclusionRemovesOnlyRejectedWorlds. Re-measured for the
		// Mausoleum Wanderer unless-cost ask label (see the test comment) and
		// again for the damage-provenance fact (see above); see the
		// pre-optimisation entry for the archetype-posterior re-measure.
		// Re-measured for cr-g67b (CR 602.2a): frames 126, accepted 25, ESS 25.
		{"land exclusion", false, "bba783b430332248182a73dc58e3dba12c31bea0d4d0556b28b592d66318cc76"},
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
	// The digest taken at 0b6e568b, before the performance work. Re-measured
	// by cli-20260922T150843Z-c6c925c4 (Counter UnlessCost$ X/SVar fold): the
	// bench fixture's Mausoleum Wanderer unless-pay ask label is part of the
	// captured frames that seed these worlds, so the sampled worlds (and
	// hence the rollout submission count) moved: measured Submits 2891
	// (pre-fix) -> 3160 (fixed). Per-candidate values, rollouts, terminal
	// counts and the 8/8/8/8 wins split are unchanged, so this is the same
	// behaviour at different world inputs, not a rollout-side change.
	// Re-measured again by cli-20260922T150843Z-daf1bd3e (the unless-pay mana
	// window), for the same reason and with the same verdict: Index, Values,
	// Rollouts, Terminal, Capped and the 8/8/8/8 wins split are all unchanged
	// and only Submits moves, 2891 -> 4054.
	// Re-measured by cli-20260922T225137Z-4a79e887 (context-resolved target
	// effects): captured target actions now carry known dynamic damage rather
	// than null, changing the sampled worlds and only Submits, 4054 -> 4000.
	// Index, Values, Rollouts, Terminal, Capped and the 8/8/8/8 wins split are
	// unchanged; this is not a rollout-side behavior change.
	// Re-measured for game-long damage-by-source provenance
	// (agent-20260923T114033Z-a57ee463): the sampler's worlds moved (above),
	// so the rollouts run on different games and only Submits moves,
	// 4000 -> 3396. Index, Values (all 1), Rollouts 32, Terminal 32,
	// Capped 0 and the 8/8/8/8 wins split are all unchanged. Measured
	// attribution: disabling ONLY the provenance emission block in
	// engine/rules/engine.go restores 20e9fd... byte-for-byte.
	// Re-measured for pn20 hindsight branch mining: TeacherResult grew
	// WinsOverBaseline/LossesToBaseline (paired terminal outcomes vs
	// candidate 0, needed for hindsight's confidence intervals), so the
	// hashed JSON gained two zero-valued array fields and only the digest
	// moves, fa6bb92b... -> d97fc053... Index, Values, Rollouts, Terminal,
	// Capped and the 8/8/8/8 wins split are unchanged (both new fields are
	// [0,0,0,0] on this fixture, since it has no repeated world/candidate-0
	// split to pair).
	// CR 309's public completed_dungeons field changes the captured-frame
	// seed and hence the sampled worlds. Index, Values, Rollouts 32,
	// Terminal 32, Capped 0 and wins 8/8/8/8 are unchanged; Submits is 3878.
	// Re-measured for the opponent archetype posterior
	// (agent-20260930T000950Z-25c1cb85), for the same reason: the captured
	// frames now marshal PlayerView.Archetype, so the sampled worlds moved
	// and only Submits moves, 3878 -> 2995. Index, Values (all 1), Rollouts
	// 32, Terminal 32, Capped 0 and the 8/8/8/8 wins split are unchanged.
	// Re-measured for CR 103.8a's turn-1 draw STEP skip (2026-10-06): the
	// sampled worlds moved with the fixture's frames (above), so only Submits
	// moves, 2995 -> 3450; Index 0, Values all 1, Rollouts 32, Terminal 32,
	// Capped 0 and the 8/8/8/8 wins split are unchanged.
	// Re-measured for the per-turn watcher view fields (eng-turnwatch): the
	// captured frames marshal four more PlayerView keys, so the sampled worlds
	// move and only Submits moves, 3450 -> 3598. Index 0, Values
	// [1,0.875,1,1], Rollouts 32, Terminal 32, Capped 0 and the 8/8/8/8
	// TerminalByCandidate split are as measured. Attributed by hiding only the
	// four view fields (json:"-"): the old digest returns byte-for-byte.
	// Re-measured for the paid-cost mana census default flip (2026-10-13): the
	// census offers pool-short abilities and counts paid-cost sources, so the
	// rollouts' games move and Submits 3598 -> 3852 with WinsByCandidate
	// [8,7,8,8] and LossesToBaseline [0,1,0,0]; Index 0, Values [1,0.875,1,1],
	// Rollouts 32, Terminal 32, Capped 0 and the 8/8/8/8 TerminalByCandidate
	// split as measured. Attributed by forcing engine/rules/genesis.go's
	// paidManaCensus to false: the old digest returns byte-for-byte.
	// Re-measured for the token-art printed-P/T field (token-art-pt-key):
	// CardView gained printed_pt, so the captured frames that seed these
	// worlds marshal one more key per token and the sampled worlds (and hence
	// the rollouts) move. Attributed by hiding only that field (json:"-"): the
	// old digest returns byte-for-byte. Measured after: Index 0, Values
	// [1,0.5,1,1], Rollouts 32, Submits 6288, Terminal 32, Capped 0,
	// WinsByCandidate [8,4,8,8], TerminalByCandidate [8,8,8,8],
	// WinsOverBaseline [0,0,0,0], LossesToBaseline [0,4,0,0].
	// Re-measured for the combat priority fixes (rules e7aaf1530 and
	// 93b6057e4): the sampled worlds moved with the fixture's frames (above),
	// so the rollouts run on different games. Not moved by the view's
	// derived type line (26ea2ed4f). Measured after: Index 0, Values
	// [1,1,1,1], Rollouts 32, Submits 3939, Terminal 32, Capped 0,
	// WinsByCandidate [8,8,8,8], TerminalByCandidate [8,8,8,8],
	// WinsOverBaseline [0,0,0,0], LossesToBaseline [0,0,0,0].
	// Re-measured for the CR 510.3 post-damage priority round (rules
	// 24bbba791), the sampled worlds again: Submits 3939 -> 3529; Index,
	// Values [1,1,1,1], Rollouts 32, Terminal 32, Capped 0 and the 8/8/8/8
	// splits unchanged.
	// Re-measured for the A2 targeting port (view.CardView.ColourIdentity,
	// json-tagged so every frame encoding gains a field): the sampled
	// worlds are decoded from those frames, so the rollouts run on
	// differently-encoded worlds. Attributed by tagging only that field
	// json:"-": the old digest and this test's old result return
	// byte-for-byte. Measured after: Index 0, Values [1,0.875,1,1],
	// Rollouts 32, Submits 4150, Terminal 32, Capped 0,
	// WinsByCandidate [8,7,8,8], TerminalByCandidate [8,8,8,8],
	// WinsOverBaseline [0,0,0,0], LossesToBaseline [0,1,0,0].
	// Re-measured for the A1 removal-now port (view.CardView.RemovalKind,
	// RemovalTargets, RemovalDamage): the captured frames marshal three more
	// keys per card, so the sampled worlds are decoded from differently
	// encoded frames and the rollouts run on different games. Attributed by
	// hiding the three fields (json:"-"): the old digest and this test's old
	// result return byte-for-byte. Measured after: Index 0, Values [1,1,1,1],
	// Rollouts 32, Submits 3625, Terminal 32, Capped 0, WinsByCandidate
	// [8,8,8,8], TerminalByCandidate [8,8,8,8], WinsOverBaseline [0,0,0,0],
	// LossesToBaseline [0,0,0,0].
	// Re-measured for the A3 Forge AI-hint port (view.CardView.PlayMain1,
	// RemoveDeck): the captured frames marshal two more keys for hinted
	// cards, so the sampled worlds are decoded from differently encoded
	// frames and the rollouts run on different games. Attributed by hiding the
	// two fields (json:"-"): the old digest and this test's old result return
	// byte-for-byte. Measured after: Index 0, Values [1,1,1,1], Rollouts 32,
	// Submits 4489, Terminal 32, Capped 0, WinsByCandidate [8,8,8,8],
	// TerminalByCandidate [8,8,8,8], WinsOverBaseline [0,0,0,0],
	// LossesToBaseline [0,0,0,0].
	// Re-measured for the A6a pump-timing port (view.CardView.PumpPower,
	// PumpToughness): the captured frames marshal two more keys per card, so
	// the sampled worlds are decoded from differently encoded frames and the
	// rollouts run on different games. Attributed by hiding the two fields
	// (json:"-"): the old digest and this test's old result return
	// byte-for-byte. Measured after: Index 0, Values [1,1,1,1], Rollouts 32,
	// Submits 3097, Terminal 32, Capped 0, WinsByCandidate [8,8,8,8],
	// TerminalByCandidate [8,8,8,8], WinsOverBaseline [0,0,0,0],
	// LossesToBaseline [0,0,0,0].
	// Re-measured for the A6a restricted-target-scope fact
	// (view.CardView.PumpOwn): one more key per card moves the decoded frames
	// and the rollouts again. Attributed by hiding that field (json:"-"): the
	// previous digest and result return byte-for-byte. Measured after:
	// Index 0, Values [1,1,1,1], Rollouts 32, Submits 3309, Terminal 32,
	// Capped 0, WinsByCandidate [8,8,8,8], TerminalByCandidate [8,8,8,8],
	// WinsOverBaseline [0,0,0,0], LossesToBaseline [0,0,0,0].
	// Re-measured for cr-g67b (CR 602.2a): the captured frames' stack moves
	// when an activation's mint precedes its first decision, so the teacher's
	// decoded frames and rollouts move with them.
	const want = "52f8e0b29b017f836f4649fcd00c5ac4516c84cb464c8bfec88012358114faba"
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
