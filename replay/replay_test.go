package replay

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
)

// maxIntents bounds every drive loop below so a genuine engine bug (an
// Advance/Submit pair that never reaches Over or a nil Pending) fails the
// test instead of hanging the one goroutine running it (global constraint:
// an unbounded loop is a DoS).
const maxIntents = 200000

// playGame drives a real 4-seat game to completion with the deterministic
// bot (Ruling P1: Task 25's seat.NewBot + testutil.SampleDecks, not a
// replay-local stand-in) and returns both the Config that produced it and
// the finished engine, so callers can read its Log, Head, and RNGDraws.
// Reusing the returned cfg for a later Replay/ReplayTo call is what gives
// the (cfg, l) contract the same deck *cards.Card pointers, in the same
// order, that Ruling P5/T11-d requires -- calling testutil.SampleDecks a
// second time would build structurally-identical but distinct *cards.Card
// values instead.
func playGame(t *testing.T, seed uint64) (rules.Config, *rules.Engine) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	cfg := rules.Config{Seed: seed, Names: names, Decks: decks}
	e := rules.New(cfg)
	e.Advance()
	b := seat.NewBot(seed)
	n := 0
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		d := e.Pending()
		in, err := botAnswer(b, e, d)
		if err != nil {
			t.Fatalf("bot Decide returned an error: %v", err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("Submit intent %d: %v", n, err)
		}
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents", n)
	}
	if len(e.L.Intents) == 0 {
		t.Fatal("recorded game made no decisions at all -- test setup is broken")
	}
	return cfg, e
}

// copyLog returns a *events.Log that shares orig's recorded Events and
// internal chain state (a plain struct copy of *orig is permitted across
// package boundaries even though events.Log carries unexported fields --
// only naming a field is restricted, not copying the whole struct) but owns
// an independent Intents slice, so a test can mutate one Intent without
// corrupting the original recording other tests may still read.
//
// Ruling T24-c (fix round 1, Important #2): copying the []decision.Intent
// slice alone is not enough -- each decision.Intent is a plain struct, so
// append-copying the slice copies every Intent's Choices SLICE HEADER, not
// its backing array. A test that then writes into one copy's
// Choices[j] (the duplicate-choice totality row did exactly this) was
// silently corrupting orig's backing array too, since both headers still
// pointed at the same underlying []int. Every Intent's Choices is now
// copied independently so mutating one copy can never reach another, or
// the original.
func copyLog(orig *events.Log) *events.Log {
	c := *orig
	c.Intents = make([]decision.Intent, len(orig.Intents))
	for i, in := range orig.Intents {
		in.Choices = append([]int(nil), in.Choices...)
		c.Intents[i] = in
	}
	return &c
}

// TestReplayReproducesTheChain is the brief's test 1: play a 4-seat game,
// replay it from (seed, intents), and assert equal Head, equal event count
// and equal RNG draw count.
func TestReplayReproducesTheChain(t *testing.T) {
	cfg, orig := playGame(t, 1)

	rep, err := Replay(orig.L, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if got, want := len(rep.L.Events), len(orig.L.Events); got != want {
		t.Errorf("event count = %d, want %d", got, want)
	}
	if got, want := rep.L.Head(), orig.L.Head(); got != want {
		t.Errorf("Head() = %s, want %s", got, want)
	}
	if got, want := rep.RNGDraws(), orig.RNGDraws(); got != want {
		t.Errorf("RNGDraws() = %d, want %d", got, want)
	}
}

// TestReplayToPrefixMatchesHeadAt is the brief's test 2: ReplayTo(l, cfg, n)
// for a midpoint n produces a log whose Head equals the original log's
// HeadAt the same number of events -- "playback to N" is verifiable against
// the full log without needing the rest of it replayed.
func TestReplayToPrefixMatchesHeadAt(t *testing.T) {
	cfg, orig := playGame(t, 2)

	n := len(orig.L.Intents) / 2
	rep, err := ReplayTo(orig.L, cfg, n)
	if err != nil {
		t.Fatalf("ReplayTo(%d): %v", n, err)
	}
	got := rep.L.Head()
	want := orig.L.HeadAt(len(rep.L.Events))
	if got != want {
		t.Errorf("Head() = %s, want HeadAt(%d) = %s", got, len(rep.L.Events), want)
	}

	// Bundled fix round 1 item: supplement §3 also says ReplayTo positions
	// the engine at the next pending decision -- the very one intent n
	// itself answers. Check that against two independent pieces of the
	// recording, not just one echoing the other: Seq and Player against the
	// recorded Intent, and Kind against the DecisionAsk event's own Text
	// (Engine.ask logs Text: string(d.Kind)) at that same Seq -- Engine.ask
	// assigns d.Seq = len(e.L.Events) before emitting DecisionAsk, so
	// l.Events[in.Seq] is exactly the ask this Intent answers.
	in := orig.L.Intents[n]
	p := rep.Pending()
	if p == nil {
		t.Fatalf("Pending() = nil, want the decision intent %d answers", n)
	}
	if p.Seq != in.Seq {
		t.Errorf("Pending().Seq = %d, want %d", p.Seq, in.Seq)
	}
	if p.Player != in.Player {
		t.Errorf("Pending().Player = %d, want %d", p.Player, in.Player)
	}
	if int(in.Seq) >= len(orig.L.Events) {
		t.Fatalf("intent %d's Seq %d is out of range of the recorded log", n, in.Seq)
	}
	askEvent := orig.L.Events[in.Seq]
	if askEvent.Kind != events.DecisionAsk {
		t.Fatalf("event at Seq %d is %s, want %s", in.Seq, askEvent.Kind, events.DecisionAsk)
	}
	if string(p.Kind) != askEvent.Text {
		t.Errorf("Pending().Kind = %s, want %s (from the recorded DecisionAsk event)", p.Kind, askEvent.Text)
	}
}

// TestResumeFromMidpointReachesTheSameEnd is the brief's test 3, plus the
// bundled fix round 1 item that gives the name a real test: the brief's own
// text uses n == len(Intents), which never resumes anything (every Intent
// fed back is one already in the log) -- so this keeps that check as its
// own assertion, then separately stops at a genuine midpoint and drives the
// returned engine onward with a DIFFERENT bot seed to a fresh, ordinary
// Over. That second half is the one thing neither this test's first half
// nor TestReplayReproducesTheChain can exercise: both of those only ever
// feed back recorded Intents, so neither ever calls Submit with anything
// ReplayTo's own engine did not already see coming.
func TestResumeFromMidpointReachesTheSameEnd(t *testing.T) {
	cfg, orig := playGame(t, 3)

	// "Playback to" and "resume from" are the same operation (Ruling P3's
	// doc on ReplayTo) with a different n: ReplayTo(l, cfg, len(l.Intents))
	// -- every recorded Intent, the same prefix Replay itself would run --
	// reaches the same Head as the original.
	full, err := ReplayTo(orig.L, cfg, len(orig.L.Intents))
	if err != nil {
		t.Fatalf("ReplayTo(full): %v", err)
	}
	if got, want := full.L.Head(), orig.L.Head(); got != want {
		t.Errorf("Head() = %s, want %s", got, want)
	}
	if got, want := len(full.L.Events), len(orig.L.Events); got != want {
		t.Errorf("event count = %d, want %d", got, want)
	}

	// The headline "resume from" behaviour: stop at a genuine midpoint and
	// continue with a fresh policy, not the recorded one.
	mid := len(orig.L.Intents) / 3
	if mid == 0 {
		t.Fatal("recorded game too short to resume from a midpoint -- test setup assumption broke")
	}
	resumed, err := ReplayTo(orig.L, cfg, mid)
	if err != nil {
		t.Fatalf("ReplayTo(%d): %v", mid, err)
	}
	if resumed.G.Over {
		t.Fatalf("game already over at intent %d -- test setup assumption broke", mid)
	}
	b := seat.NewBot(cfg.Seed + 777) // deliberately different from cfg.Seed's own bot.
	n := 0
	for !resumed.G.Over && resumed.Pending() != nil && n < maxIntents {
		d := resumed.Pending()
		in, err := botAnswer(b, resumed, d)
		if err != nil {
			t.Fatalf("bot Decide returned an error: %v", err)
		}
		if err := resumed.Submit(in); err != nil {
			t.Fatalf("Submit intent %d after resuming: %v", n, err)
		}
		n++
	}
	if !resumed.G.Over {
		t.Fatalf("resumed game did not terminate after %d further intents", n)
	}
}

// decisionMadeSeqs returns, in order, the Seq (== slice index; Log.Append
// assigns Seq from the then-current length) of every DecisionMade event in
// l -- one per recorded Intent, in the same order, since Engine.Submit
// appends to L.Intents and emits exactly one DecisionMade event per
// successful call. Index k of the result is therefore the event altering
// l.Intents[k] is guaranteed to change first.
func decisionMadeSeqs(l *events.Log) []uint64 {
	var seqs []uint64
	for i, ev := range l.Events {
		if ev.Kind == events.DecisionMade {
			seqs = append(seqs, uint64(i))
		}
	}
	return seqs
}

// TestDivergenceNamesTheFirstBadEvent is the brief's test 4: alter one
// recorded Intent's choices and confirm Replay reports a *Divergence naming
// the first event that actually differs.
//
// The DecisionMade event Engine.Submit emits encodes in.Choices directly in
// its Text field (fmt.Sprintf("%s:%v", d.Kind, in.Choices)), so changing an
// Intent's choice value -- to another value still valid for the SAME
// decision, so Decision.Validate still accepts it -- guarantees the very
// next event produced differs from the recording, regardless of whether the
// altered choice has any further, semantic effect on the game. Everything
// before that Intent replays identically (nothing has diverged yet), so
// this is a controlled, single-point change: exactly what the test needs to
// pin the reported Seq against.
func TestDivergenceNamesTheFirstBadEvent(t *testing.T) {
	cfg, orig := playGame(t, 4)

	// Find an Intent with exactly one choice, at a nonzero index. Its
	// decision therefore offered at least two options (index 0 and the
	// recorded one), so rewriting the choice to 0 is guaranteed to still be
	// in range, still a single distinct choice, and still for the same
	// player -- Validate will accept it -- while producing a different
	// DecisionMade Text than what was recorded.
	k := -1
	for i, in := range orig.L.Intents {
		if len(in.Choices) == 1 && in.Choices[0] != 0 {
			k = i
			break
		}
	}
	if k < 0 {
		t.Fatal("no single-choice, nonzero-index Intent found to alter -- test setup assumption broke")
	}

	seqs := decisionMadeSeqs(orig.L)
	if k >= len(seqs) {
		t.Fatalf("intent index %d has no matching DecisionMade event (found %d)", k, len(seqs))
	}
	wantSeq := seqs[k]

	altered := copyLog(orig.L)
	altered.Intents[k].Choices = []int{0}

	_, err := Replay(altered, cfg)
	if err == nil {
		t.Fatal("Replay succeeded despite an altered Intent; want a *Divergence")
	}
	var div *Divergence
	if !errors.As(err, &div) {
		t.Fatalf("error = %v (%T), want a *Divergence", err, err)
	}
	if div.Missing {
		t.Fatalf("Divergence.Missing = true at Seq %d, want a content mismatch at Seq %d", div.Seq, wantSeq)
	}
	if div.Seq != wantSeq {
		t.Errorf("Divergence.Seq = %d, want %d (the DecisionMade event for the altered intent)", div.Seq, wantSeq)
	}
	if div.Want.Kind != events.DecisionMade || div.Got.Kind != events.DecisionMade {
		t.Errorf("Want.Kind/Got.Kind = %s/%s, want both %s", div.Want.Kind, div.Got.Kind, events.DecisionMade)
	}
}

// TestReplayRejectsAnIntentTheEngineWouldNotAskFor is the brief's test 5,
// tightened by Ruling P2: an Intent whose Seq does not match the pending
// decision must be refused with a clear error, not silently repaired (by
// rewriting its Seq to match) and applied anyway. The refusal is a plain
// wrapped error, not a *Divergence -- nothing was ever compared, because
// Submit rejected the Intent before producing anything for compare to see.
func TestReplayRejectsAnIntentTheEngineWouldNotAskFor(t *testing.T) {
	cfg, orig := playGame(t, 5)

	altered := copyLog(orig.L)
	altered.Intents[0].Seq += 999 // guaranteed to mismatch whatever Seq is actually pending.

	_, err := Replay(altered, cfg)
	if err == nil {
		t.Fatal("Replay succeeded despite a Seq the engine never offered")
	}
	var div *Divergence
	if errors.As(err, &div) {
		t.Fatalf("got a *Divergence (%v); want a plain rejection -- nothing should have been compared", div)
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Errorf("error = %q, want it to say the intent was rejected", err.Error())
	}
}

// TestReplayTotality is the supplement's §6 table (Ruling P4): Replay and
// ReplayTo re-execute from (cfg, intents) rather than Apply a logged event
// directly, so a tampered or malformed log can only ever produce an error
// (a *Divergence or a plain rejection) -- never a panic, no matter how the
// log or cfg is broken. Every row below is one way to break it; none must
// panic, and each must return either an error or a usable (non-nil) engine.
func TestReplayTotality(t *testing.T) {
	cfg, orig := playGame(t, 6)

	nilIntents := copyLog(orig.L)
	nilIntents.Intents = nil

	outOfRange := copyLog(orig.L)
	outOfRange.Intents[0].Choices = []int{1 << 20}

	negative := copyLog(orig.L)
	negative.Intents[0].Choices = []int{-1}

	// Find an Intent with 2+ choices (a multi-attacker declaration or a
	// trigger-order permutation) so duplicating one entry is possible.
	dupIdx := -1
	for i, in := range orig.L.Intents {
		if len(in.Choices) >= 2 {
			dupIdx = i
			break
		}
	}
	if dupIdx < 0 {
		t.Fatal("no multi-choice Intent found in the recorded game -- test setup assumption broke")
	}
	duplicate := copyLog(orig.L)
	duplicate.Intents[dupIdx].Choices[1] = duplicate.Intents[dupIdx].Choices[0]

	wrongPlayer := copyLog(orig.L)
	wrongPlayer.Intents[0].Player++

	fewerNames, fewerDecks := testutil.SampleDecks(t, 2)
	seatMismatchCfg := rules.Config{Seed: cfg.Seed, Names: fewerNames, Decks: fewerDecks}

	cases := []struct {
		name string
		run  func() (*rules.Engine, error)
		// check, when non-nil, replaces the default "no panic, not both
		// nil" assertion with a stronger, row-specific one.
		check func(t *testing.T, e *rules.Engine, err error)
	}{
		{"nil log via Replay", func() (*rules.Engine, error) { return Replay(nil, cfg) }, nil},
		{"nil log via ReplayTo", func() (*rules.Engine, error) { return ReplayTo(nil, cfg, 1) }, nil},
		{"nil Intents", func() (*rules.Engine, error) { return Replay(nilIntents, cfg) }, nil},
		{"out-of-range choice", func() (*rules.Engine, error) { return Replay(outOfRange, cfg) }, nil},
		{"negative choice", func() (*rules.Engine, error) { return Replay(negative, cfg) }, nil},
		{"duplicate choice", func() (*rules.Engine, error) { return Replay(duplicate, cfg) }, nil},
		{"wrong player", func() (*rules.Engine, error) { return Replay(wrongPlayer, cfg) }, nil},
		{"seat count mismatch", func() (*rules.Engine, error) { return Replay(orig.L, seatMismatchCfg) }, nil},
		{"ReplayTo n negative", func() (*rules.Engine, error) { return ReplayTo(orig.L, cfg, -5) }, nil},
		// Ruling T24-c (fix round 1, Important #2): this row shares orig.L
		// with every hostile row above via copyLog. It is the canary for a
		// repeat of the Choices-aliasing bug that let the duplicate-choice
		// row above corrupt orig.L's own backing array -- a corrupted
		// orig.L would make THIS row stop partway through with a plain
		// rejection instead of completing, which "no panic and not both
		// nil" alone would not catch (it did not, the first time). Assert
		// the strong property instead: a clean, full, chain-matching
		// replay of the untouched original log.
		{"ReplayTo n too large", func() (*rules.Engine, error) {
			return ReplayTo(orig.L, cfg, len(orig.L.Intents)+1000)
		}, func(t *testing.T, e *rules.Engine, err error) {
			if err != nil {
				t.Fatalf("ReplayTo(len+1000) against the untouched original log: %v", err)
			}
			if got, want := e.L.Head(), orig.L.Head(); got != want {
				t.Errorf("Head() = %s, want %s (a clean full replay of the original log)", got, want)
			}
			if got, want := len(e.L.Events), len(orig.L.Events); got != want {
				t.Errorf("event count = %d, want %d", got, want)
			}
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked: %v", r)
				}
			}()
			e, err := c.run()
			if c.check != nil {
				c.check(t, e, err)
				return
			}
			if err == nil && e == nil {
				t.Fatal("returned neither an error nor an engine")
			}
		})
	}
}

// TestDivergenceOnTheVeryLastEvent is a self-review regression: the
// compare loop's own bounds (`i < len(got)`, `i >= len(l.Events)`) must
// catch a mismatch at the LAST recorded event exactly as readily as one in
// the middle -- an off-by-one here would either miss the final event
// entirely or report a Divergence one Seq short/long. Truncating the
// recorded log by exactly one event, with every Intent left untouched and
// therefore replaying identically right up to that point, forces the
// mismatch to land exactly at the truncated log's last valid index: the
// replay reproduces every earlier event and then, on the very next Submit,
// produces one the truncated log has no room left for at all (the Missing
// case, Ruling P3), at Seq == len(truncated.Events).
func TestDivergenceOnTheVeryLastEvent(t *testing.T) {
	cfg, orig := playGame(t, 8)
	if len(orig.L.Events) < 2 {
		t.Fatal("recorded game logged fewer than 2 events -- test setup assumption broke")
	}

	truncated := copyLog(orig.L)
	truncated.Events = append([]events.Event(nil), orig.L.Events[:len(orig.L.Events)-1]...)
	wantSeq := uint64(len(truncated.Events))

	_, err := Replay(truncated, cfg)
	if err == nil {
		t.Fatal("Replay succeeded against a log truncated by one event; want a *Divergence")
	}
	var div *Divergence
	if !errors.As(err, &div) {
		t.Fatalf("error = %v (%T), want a *Divergence", err, err)
	}
	if !div.Missing {
		t.Fatalf("Divergence.Missing = false, want true (replay ran past the truncated log's end)")
	}
	if div.Seq != wantSeq {
		t.Errorf("Divergence.Seq = %d, want %d (the truncated log's own length)", div.Seq, wantSeq)
	}
}

// TestReplayToClampsOutOfRangeN documents ReplayTo's totality behaviour
// (Ruling P4) beyond "does not panic": n < 0 behaves as n == 0 (genesis
// only, no Intents applied), and n > len(l.Intents) behaves as the full
// replay -- both are a clean prefix engine, not an error.
func TestReplayToClampsOutOfRangeN(t *testing.T) {
	cfg, orig := playGame(t, 7)

	zero, err := ReplayTo(orig.L, cfg, 0)
	if err != nil {
		t.Fatalf("ReplayTo(0): %v", err)
	}
	neg, err := ReplayTo(orig.L, cfg, -5)
	if err != nil {
		t.Fatalf("ReplayTo(-5): %v", err)
	}
	if got, want := neg.L.Head(), zero.L.Head(); got != want {
		t.Errorf("ReplayTo(-5).Head() = %s, want ReplayTo(0)'s %s", got, want)
	}

	full, err := ReplayTo(orig.L, cfg, len(orig.L.Intents))
	if err != nil {
		t.Fatalf("ReplayTo(full): %v", err)
	}
	over, err := ReplayTo(orig.L, cfg, len(orig.L.Intents)+1000)
	if err != nil {
		t.Fatalf("ReplayTo(over): %v", err)
	}
	if got, want := over.L.Head(), full.L.Head(); got != want {
		t.Errorf("ReplayTo(over).Head() = %s, want ReplayTo(full)'s %s", got, want)
	}
}

// TestReplayVerifiesADeserialisedLog is fix round 1's Important #1
// (Ruling T24-a): events.Log.chain is unexported with no json tag, while
// Seed, Events and Intents all have one, so a Log that came off disk or the
// wire has a zero chain and Head() reports the seedless zero hash -- even
// when every event matches the replay byte for byte. Replay must verify
// such a log against l.HeadAt(len(l.Events)), which recomputes the chain
// from Seed and Events alone, not against l.Head() directly.
//
// The deliberately wrong cfg.Seed is not the point of this test (Ruling P5
// and TestReplayReproducesTheChain-adjacent coverage already establish that
// l.Seed wins); it is here only to make sure the replay's success is not
// somehow an artifact of the seed happening to match.
func TestReplayVerifiesADeserialisedLog(t *testing.T) {
	cfg, orig := playGame(t, 9)

	raw, err := json.Marshal(orig.L)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var deserialised events.Log
	if err := json.Unmarshal(raw, &deserialised); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	// Confirm the fixture actually exercises the zero-chain case: if this
	// ever stops being the zero chain (e.g. events.Log grows a json tag on
	// its chain field), this test would otherwise pass for the wrong
	// reason.
	if got, want := deserialised.Head(), "0000000000000000"; got != want {
		t.Fatalf("test fixture assumption broke: deserialised.Head() = %q, want the zero chain %q", got, want)
	}

	wrongSeedCfg := cfg
	wrongSeedCfg.Seed = cfg.Seed + 12345

	rep, err := Replay(&deserialised, wrongSeedCfg)
	if err != nil {
		t.Fatalf("Replay of a JSON round-tripped log: %v", err)
	}
	if got, want := len(rep.L.Events), len(orig.L.Events); got != want {
		t.Errorf("event count = %d, want %d", got, want)
	}
	if got, want := rep.L.Head(), orig.L.HeadAt(len(orig.L.Events)); got != want {
		t.Errorf("Head() = %s, want %s", got, want)
	}
}

// TestReplayVerifiesANoHashLog covers the other half of Important #1:
// HeadAt (like Head) short-circuits to "" for a NoHash log (events/log.go),
// and the replay's own freshly-built log is always hashing (rules.Config
// carries no NoHash field for New to propagate), so comparing the two
// directly would fail every NoHash log for a reason that has nothing to do
// with whether it replayed correctly. Replay must skip that comparison
// rather than fail it.
//
// There is no way to produce a genuinely NoHash-recorded game through the
// public API without a rules.Config field this fix round is not scoped to
// add (NoHash must be set before a Log's first Append, and rules.New's own
// genesis already appends before returning), so this flips the flag on a
// full copy of an ordinarily-recorded log instead. NoHash only changes
// Log.Append's own chain bookkeeping, never the Events/Intents content, so
// this is a faithful way to exercise the code path in Replay that reads
// l.NoHash without needing an engine change.
func TestReplayVerifiesANoHashLog(t *testing.T) {
	cfg, orig := playGame(t, 10)

	noHash := copyLog(orig.L)
	noHash.NoHash = true
	if got := noHash.HeadAt(len(noHash.Events)); got != "" {
		t.Fatalf("test fixture assumption broke: HeadAt on a NoHash log = %q, want \"\"", got)
	}

	rep, err := Replay(noHash, cfg)
	if err != nil {
		t.Fatalf("Replay of a NoHash log: %v", err)
	}
	if got, want := len(rep.L.Events), len(orig.L.Events); got != want {
		t.Errorf("event count = %d, want %d", got, want)
	}
}
