package host

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
)

// burstObs is one observed burst: the events slice and the intent that drove
// it (nil for the genesis burst).
type burstObs struct {
	evs []events.Event
	in  *decision.Intent
}

// TestOnBurstSinksTheWholeChainAndReplaysToTheSameHead is Task M2c-1's point:
// an embedder's OnBurst/OnMatchEnd sink must see every event and intent of an
// in-memory (Dir="") bot match, in chain order from genesis through the final
// GameOver, with OnMatchEnd firing exactly once carrying the finished
// MatchInfo — and the sink's data alone must replay to the same chain head the
// match produced. An observer that perturbs the game is worse than none; if
// the sink dropped, reordered or altered anything, the replay below diverges
// or its head changes.
//
// The only data sourced from the finished match (rather than the sink) is the
// game setup a replay needs — cfg's resolved decks/names — and the seed for
// comparison. The full event stream, including genesis, comes from the sink.
func TestOnBurstSinksTheWholeChainAndReplaysToTheSameHead(t *testing.T) {
	t.Parallel()
	var (
		bursts   []burstObs
		obsK     int // the match number each hook saw (k is how an embedder keys its rows)
		endK     int
		endCount int
		endInfo  protocol.MatchInfo
	)
	o := testOptions(t)
	o.OnBurst = func(_ TableID, k int, evs []events.Event, in *decision.Intent) error {
		obsK = k
		bursts = append(bursts, burstObs{evs: append([]events.Event(nil), evs...), in: in})
		return nil
	}
	o.OnMatchEnd = func(_ TableID, k int, m protocol.MatchInfo) error {
		endK = k
		endCount++
		endInfo = m
		return nil
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	m := r.tables["t1"].history[0]
	r.mu.RUnlock()

	if obsK != 1 || endK != 1 {
		t.Fatalf("OnBurst k=%d OnMatchEnd k=%d, want both == 1 (match number plumbing)", obsK, endK)
	}
	if endCount != 1 {
		t.Fatalf("OnMatchEnd fired %d times, want exactly 1", endCount)
	}
	if endInfo.State != protocol.MatchFinished || endInfo.Head == "" || endInfo.Match != 1 {
		t.Fatalf("OnMatchEnd info = %+v, want an finished match 1", endInfo)
	}
	want := m.info()
	if endInfo.Head != want.Head || endInfo.Events != want.Events || endInfo.Turns != want.Turns ||
		endInfo.Result != want.Result || endInfo.Seed != want.Seed {
		t.Fatalf("OnMatchEnd info %+v disagrees with match %+v", endInfo, want)
	}

	// Stitch the observed bursts back into one stream, and the intents.
	var gotEvs []events.Event
	var gotIns []decision.Intent
	for _, b := range bursts {
		gotEvs = append(gotEvs, b.evs...)
		if b.in != nil {
			gotIns = append(gotIns, *b.in)
		}
	}
	// The genesis burst (no intent) plus one burst per recorded decision.
	if len(bursts) != 1+len(m.e.L.Intents) {
		t.Fatalf("saw %d bursts, want genesis + %d intents", len(bursts), len(m.e.L.Intents))
	}
	if len(gotEvs) != len(m.e.L.Events) {
		t.Fatalf("sink saw %d events, match log has %d", len(gotEvs), len(m.e.L.Events))
	}
	if gotEvs[0].Kind != m.e.L.Events[0].Kind || gotEvs[0].Seq != m.e.L.Events[0].Seq {
		t.Fatalf("sink's first event %+v is not the log's ledger start %+v", gotEvs[0], m.e.L.Events[0])
	}
	if gotEvs[len(gotEvs)-1].Kind != events.GameOver {
		t.Fatalf("sink's last event kind %v, want the final GameOver", gotEvs[len(gotEvs)-1].Kind)
	}
	if !reflect.DeepEqual(gotEvs, m.e.L.Events) {
		t.Fatalf("sink's burst stream is not the match log, event by event in chain order")
	}
	if !reflect.DeepEqual(gotIns, m.e.L.Intents) {
		t.Fatalf("sink's intents %+v disagree with the match's %+v", gotIns, m.e.L.Intents)
	}

	// Determinism companion: feed the sink's data alone back to replay.Replay
	// and assert it yields the same chain head. NewLog(m.seed) + Append of the
	// events reproduces the exact chain Hashes, and the sink held every
	// intent that drove it, so any perturbation by the observer shows up here.
	l := events.NewLog(m.seed)
	for _, ev := range gotEvs {
		l.Append(ev)
	}
	l.Intents = gotIns
	rep, err := replay.Replay(l, m.cfg)
	if err != nil {
		t.Fatalf("replay of sink log diverged: %v", err)
	}
	if got, want := rep.L.Head(), m.e.L.Head(); got != want {
		t.Fatalf("sink replay head %s, match head %s", got, want)
	}
}

// TestOnBurstErrorCrashesTheMatch pins D15's error path for the observer
// hook: an OnBurst error must crash the match exactly as a persist failure
// does — the match ends in MatchCrashed and the table halts — and must never
// silently continue or deadlock the match loop (which is why the callback
// runs under the match lock and must not re-enter the Registry; see
// OnBurstFunc's doc).
func TestOnBurstErrorCrashesTheMatch(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	calls := 0
	o.OnBurst = func(_ TableID, _ int, _ []events.Event, _ *decision.Intent) error {
		calls++
		if calls >= 3 {
			return errors.New("boom: third burst")
		}
		return nil
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	if got := r.Tables()[0].State; got != protocol.TableHalted {
		t.Fatalf("table state %s, want halted", got)
	}
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("match after OnBurst error: %+v, want one crashed match", ms)
	}
}

// TestGenesisOnBurstErrorCrashesTheMatch pins the earlier, distinct crash
// path an embedder's OnBurst error can hit: the genesis burst is delivered
// from play() before r.opts.Seats has run and before m.slots is set, so a
// sink failing on its very first call crashes a match that has not finished
// starting — a different site of code from the per-Submit afterBurst path.
// The ordering makes it worth a real run rather than a shrug: observeBurst
// runs inside m.locked (holding m.mu) and crash re-takes m.mu, so a deadlock
// here would leave Close/Wait hanging.
func TestGenesisOnBurstErrorCrashesTheMatch(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	calls := 0
	o.OnBurst = func(_ TableID, _ int, _ []events.Event, _ *decision.Intent) error {
		calls++
		return errors.New("boom: genesis burst")
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	// Reaching here proves nothing deadlocked on the match lock (crash
	// re-took it inside observeBurst's m.locked frame).
	if calls != 1 {
		t.Fatalf("OnBurst called %d times, want exactly the one genesis call", calls)
	}
	if got := r.Tables()[0].State; got != protocol.TableHalted {
		t.Fatalf("table state %s, want halted", got)
	}
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("match after genesis OnBurst error: %+v, want one crashed match", ms)
	}
	// Close must return, not hang, even though the crashed match never
	// finished starting.
	if err := r.Close(); err != nil {
		t.Fatalf("Close after genesis crash: %v", err)
	}
}

// TestOnMatchEndFiresForARestartedAbortedMatch pins the restart.go half of
// Task M2c-1: a match the previous process was cut off in (sidecar still
// MatchLive on disk) is rewritten aborted during a restart, and an embedder
// that persists every match must observe that terminal transition — OnMatchEnd
// fires once with the aborted MatchInfo, exactly as it would for a live match.
func TestOnMatchEndFiresForARestartedAbortedMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a registry and a match that was cut off mid-live by hand.
	r, _ := New(diskOptions(t, dir))
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if err := os.MkdirAll(filepath.Join(dir, "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar{Table: "t1", Match: 1, Seed: MatchSeed(99, 1), State: protocol.MatchLive, Spectator: "omniscient"}
	raw, _ := json.MarshalIndent(live, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "t1", "1.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t1", "1.events"),
		[]byte(`{"seq":0,"kind":0,"amount":4}`+"\n"+`{"seq":1,"kind":1,`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t1", "1.intents"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rewriteTablesJSON(t, dir, 1)

	var ends []protocol.MatchInfo
	o := diskOptions(t, dir)
	o.OnMatchEnd = func(_ TableID, _ int, m protocol.MatchInfo) error {
		ends = append(ends, m)
		return nil
	}
	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	if len(ends) != 1 || ends[0].State != protocol.MatchAborted || ends[0].Match != 1 {
		t.Fatalf("OnMatchEnd on restart = %+v, want one aborted match 1", ends)
	}
}
