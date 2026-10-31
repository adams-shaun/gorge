package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// corpusReady resolves the -dir the run() tests need via the repo's own
// helper: the main checkout's .cards (a skip, not an error, on a fresh
// clone that never fetched), which also keeps the test-cache key stable.
func corpusReady(t *testing.T) string {
	t.Helper()
	return testutil.CorpusDirOrSkip(t)
}

// TestDraftsimEventRunsAndVerifies is the end-to-end check: one FDN event with
// a fixed seed, four `bot` personas, played end to end with -verify
// semantics inside the per-test budget. It asserts the event reached
// COMPLETE, its log replays byte-identically, every game's chain head
// verified, and the standings table has the pod's four rows.
func TestDraftsimEventRunsAndVerifies(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set:     "FDN",
		seed:    1,
		events:  1,
		players: "bot,bot,bot,bot",
		drafter: "rating-colour",
		builder: "best23",
		verify:  true,
		dir:     dir,
	}
	res, err := run(o, &strings.Builder{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.events) != 1 {
		t.Fatalf("run produced %d events, want 1", len(res.events))
	}
	er := res.events[0]

	// Precondition: the log parses, starts with EventCreated and ends with
	// an EventEnded that carries the pod's four rows — the assertions below
	// all read that shape.
	entries, err := event.ParseLog(er.log)
	if err != nil {
		t.Fatalf("event log does not parse: %v", err)
	}
	if len(entries) == 0 || entries[0].Kind != event.KindEventCreated {
		t.Fatalf("first log entry kind is %q, want EventCreated", firstKind(entries))
	}
	last := entries[len(entries)-1]
	if last.Kind != event.KindEventEnded || len(last.Standings) != 4 {
		t.Fatalf("last log entry is %s with %d standings rows, want EventEnded with 4", last.Kind, len(last.Standings))
	}

	// The event is complete: replaying the log alone reaches COMPLETE.
	ev, err := event.ReplayLog(er.log)
	if err != nil {
		t.Fatalf("event log replay: %v", err)
	}
	if !ev.Done() {
		t.Fatalf("replayed event is in phase %s, want COMPLETE", ev.Phase())
	}

	// Three round-robin rounds, two Bo1 games each, every head non-empty.
	if len(er.games) != 6 {
		t.Fatalf("event played %d games, want 6 (three rounds, two series each)", len(er.games))
	}
	heads := map[string]bool{}
	for i, g := range er.games {
		if g.head == "" {
			t.Fatalf("game %d has an empty chain head", i)
		}
		if heads[g.head] {
			t.Fatalf("game %d repeats chain head %s", i, g.head)
		}
		heads[g.head] = true
		if g.round < 1 || g.round > 3 {
			t.Fatalf("game %d is round %d, want 1..3", i, g.round)
		}
	}

	// The standings have exactly the pod's seats, one row each.
	seen := map[int]bool{}
	for _, s := range er.standings {
		if seen[s.Seat] {
			t.Fatalf("standings repeat seat %d", s.Seat)
		}
		seen[s.Seat] = true
	}
	if len(seen) != 4 {
		t.Fatalf("standings cover %d seats, want 4", len(seen))
	}
}

// firstKind is the first entry's kind, or "" when there is none.
func firstKind(entries []event.Entry) event.Kind {
	if len(entries) == 0 {
		return ""
	}
	return entries[0].Kind
}

// TestDraftsimCollationIsSelectableAndValidated pins the flag contract:
// an unknown -collation is refused before anything runs, naming the
// collations the set carries, and -collation play runs a real event — its
// line names the collation and its packs come off the play config.
func TestDraftsimCollationIsSelectableAndValidated(t *testing.T) {
	dir := corpusReady(t)
	// Precondition: the two collations the flag may name are the set's own.
	names := setdata.Collations("FDN")
	if len(names) != 2 || names[0] != setdata.Play || names[1] != setdata.Arena {
		t.Fatalf("precondition: FDN carries collations %v, want [play play-arena]", names)
	}
	o := options{
		set: "FDN", collation: "paper", seed: 1, events: 1,
		players: "bot,bot,bot,bot", drafter: "rating-colour", builder: "best23", dir: dir,
	}
	if _, err := run(o, &strings.Builder{}); err == nil {
		t.Fatal("run accepted an unknown -collation")
	} else if !strings.Contains(err.Error(), `no "paper" collation`) || !strings.Contains(err.Error(), "play, play-arena") {
		t.Errorf("unknown-collation error = %q, want it to name the collation and list the set's", err)
	}

	// The default (unset) is play-arena; the printed event line says so.
	var out strings.Builder
	o.collation = ""
	if _, err := run(o, &out); err != nil {
		t.Fatalf("run at the default collation: %v", err)
	}
	if !strings.Contains(out.String(), "set FDN (play-arena)") {
		t.Errorf("default-collation event line misses `set FDN (play-arena)`; got:\n%s", out.String())
	}

	// -collation play runs a full event too, named on its line.
	out.Reset()
	o.collation = "play"
	o.verify = true
	if _, err := run(o, &out); err != nil {
		t.Fatalf("run at -collation play: %v", err)
	}
	if !strings.Contains(out.String(), "set FDN (play)") {
		t.Errorf("play-collation event line misses `set FDN (play)`; got:\n%s", out.String())
	}
}

// TestDraftsimRefusesEnvPolicy pins the rule: a -players entry
// whose bots.Info.Env is true (it searches over every seat's REAL
// decklist) refuses to seat, naming the rule, before anything runs.
func TestDraftsimRefusesEnvPolicy(t *testing.T) {
	o := options{
		set:     "FDN",
		seed:    1,
		events:  1,
		players: "sb-tactical,bot,bot,bot",
		drafter: "rating-colour",
		builder: "best23",
		dir:     ".",
	}
	_, err := run(o, &strings.Builder{})
	if err == nil {
		t.Fatal("run accepted sb-tactical, want a refusal")
	}
	if !strings.Contains(err.Error(), "bots.Info.Env") {
		t.Fatalf("refusal %q does not name the Env rule", err)
	}

	// An unknown policy name refuses the same way, naming the seat.
	o.players = "bot,bot,bot,nosuchpolicy"
	_, err = run(o, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "unknown bot policy") {
		t.Fatalf("unknown policy error = %v, want the bots.Normalize refusal", err)
	}
}

// TestDraftsimVerifyDetectsDivergence proves -verify can fail: a log whose
// GameEnded head was altered must be rejected naming the game. No games
// are played beyond the one run that records the pristine log.
func TestDraftsimVerifyDetectsDivergence(t *testing.T) {
	dir := corpusReady(t)
	o := options{set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", dir: dir}
	res, err := run(o, &strings.Builder{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	logBytes := res.events[0].log

	r, err := newTestRunner(dir, "bot,bot,bot,bot")
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	if err := verifyEvent(r, 1, logBytes); err != nil {
		t.Fatalf("pristine log failed verification: %v", err)
	}

	// Corrupt the first GameEnded head and expect the mismatch to name the
	// game. Precondition: the log really carries a GameEnded line.
	corrupt := append([]byte(nil), logBytes...)
	hit := false
	for _, line := range bytes.Split(corrupt, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"kind":"GameEnded"`)) {
			continue
		}
		idx := bytes.Index(line, []byte(`"head":"`))
		if idx < 0 {
			t.Fatalf("GameEnded line carries no head: %s", line)
		}
		line[idx+8] = '0'
		hit = true
		break
	}
	if !hit {
		t.Fatal("no GameEnded log line found to corrupt; the precondition failed")
	}
	err = verifyEvent(r, 1, corrupt)
	if err == nil {
		t.Fatal("verifyEvent accepted a corrupted GameEnded head")
	}
	if !strings.Contains(err.Error(), "replayed chain head") {
		t.Fatalf("divergence error %q does not name the chain-head comparison", err)
	}
}

// newTestRunner builds a runner the way run() does, for the verify tests.
func newTestRunner(dir, players string) (*runner, error) {
	spec, err := setdata.Load("FDN")
	if err != nil {
		return nil, err
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return nil, err
	}
	list := strings.Split(players, ",")
	lookup := func(name string) *cards.Card { c, _ := reg.Lookup(name); return c }
	drafter, builder, err := personaPair("rating-colour", "best23", lookup)
	if err != nil {
		return nil, err
	}
	return &runner{
		reg:     reg,
		spec:    spec,
		drafter: drafter,
		builder: builder,
		players: list,
		out:     &strings.Builder{},
	}, nil
}
