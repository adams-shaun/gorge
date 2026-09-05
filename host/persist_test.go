package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func diskOptions(t *testing.T, dir string) Options {
	o := testOptions(t)
	o.Dir = dir
	return o
}

func playOneToDisk(t *testing.T, dir string) {
	t.Helper()
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
}

func TestFilesAreWrittenAndByteIdenticalAcrossRuns(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	playOneToDisk(t, a)
	playOneToDisk(t, b)
	for _, name := range []string{"tables.json", "t1/1.events", "t1/1.intents", "t1/1.json"} {
		x, err := os.ReadFile(filepath.Join(a, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		y, _ := os.ReadFile(filepath.Join(b, name))
		if string(x) != string(y) {
			t.Fatalf("%s differs between two runs of the same configuration", name)
		}
		if len(x) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	var sc sidecar
	raw, _ := os.ReadFile(filepath.Join(a, "t1/1.json"))
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.State != protocol.MatchFinished || sc.Head == "" || sc.Events < 100 || sc.Spectator != "omniscient" || len(sc.Decks) != 4 {
		t.Fatalf("sidecar %+v", sc)
	}
	var tj struct {
		Tables []struct {
			Config TableConfig `json:"config"`
			Match  int         `json:"match"`
		} `json:"tables"`
	}
	raw, _ = os.ReadFile(filepath.Join(a, "tables.json"))
	if err := json.Unmarshal(raw, &tj); err != nil {
		t.Fatal(err)
	}
	if len(tj.Tables) != 1 || tj.Tables[0].Match != 1 || tj.Tables[0].Config.Spectator != view.Omniscient {
		t.Fatalf("tables.json %+v", tj)
	}
	if !strings.Contains(string(raw), `"spectator": "omniscient"`) {
		t.Fatalf("spectator is not serialised as its name:\n%s", raw)
	}
}

func TestAFinishedMatchIsServedFromDiskAfterRestart(t *testing.T) {
	dir := t.TempDir()
	playOneToDisk(t, dir)
	var liveHead string
	{
		r, _ := New(diskOptions(t, dir))
		ms, _ := r.Matches("t1")
		liveHead = ms[0].Head
		r.Close()
	}
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	tabs := r.Tables()
	if len(tabs) != 1 || tabs[0].Match != 1 || tabs[0].State != protocol.TableIdle {
		t.Fatalf("tables after restart %+v", tabs)
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished || ms[0].Head != liveHead {
		t.Fatalf("matches after restart %+v, %v", ms, err)
	}
	evs, err := r.Events("t1", 1, 0)
	if err != nil || len(evs) != ms[0].Events {
		t.Fatalf("events after restart: %d, %v", len(evs), err)
	}
	v, err := r.ViewAt("t1", 1, uint64(ms[0].Events-1))
	if err != nil || !v.Over {
		t.Fatalf("ViewAt head after restart: over=%v, %v", v.Over, err)
	}
	mid, err := r.ViewAt("t1", 1, uint64(ms[0].Events/2))
	if err != nil || mid.Over || mid.Turn == 0 {
		t.Fatalf("ViewAt mid after restart: %+v, %v", mid, err)
	}
}

func TestRestartMarksLiveMatchesAbortedAndContinuesAPerpetualTable(t *testing.T) {
	dir := t.TempDir()
	// Simulate a process that died mid-match: write the registry and a live
	// sidecar by hand, the same shape the host writes.
	r, _ := New(diskOptions(t, dir))
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if err := os.MkdirAll(filepath.Join(dir, "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar{Table: "t1", Match: 3, Seed: MatchSeed(99, 3), State: protocol.MatchLive, Spectator: "omniscient"}
	raw, _ := json.MarshalIndent(live, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.json"), raw, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.events"), []byte(`{"seq":0,"kind":0,"amount":4}`+"\n"+`{"seq":1,"kind":1,`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.intents"), nil, 0o644)
	rewriteTablesJSON(t, dir, 3)

	stops := 0
	o := diskOptions(t, dir)
	var r2 *Registry
	o.Cooldown = 1
	o.Sleep = func(d time.Duration, _ <-chan struct{}) {
		if d == 1 {
			stops++
			if stops == 1 {
				// Close is asynchronous (it waits for this very goroutine),
				// so wait for its stop signal before returning, or the loop
				// could start match 5 first (the same pattern as
				// TestAPerpetualTableStartsTheNextMatchWithTheDerivedSeed).
				go r2.Close()
				<-r2.Done()
			}
		}
	}
	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	ms, _ := r2.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted || ms[0].Match != 3 {
		t.Fatalf("after restart: %+v", ms)
	}
	if err := r2.StartAll(); err != nil {
		t.Fatal(err)
	}
	r2.Wait("t1")
	ms, _ = r2.Matches("t1")
	if len(ms) != 2 || ms[1].Match != 4 || ms[1].Seed != MatchSeed(99, 4) || ms[1].State != protocol.MatchFinished {
		t.Fatalf("perpetual table did not continue at k+1: %+v", ms)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, "t1", "3.json"))
	if !strings.Contains(string(raw), `"aborted"`) {
		t.Fatalf("sidecar 3 not rewritten as aborted:\n%s", raw)
	}
}

// rewriteTablesJSON sets the recorded match index for t1.
func rewriteTablesJSON(t *testing.T, dir string, k int) {
	t.Helper()
	p := filepath.Join(dir, "tables.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var tj struct {
		Tables []struct {
			Config TableConfig `json:"config"`
			Match  int         `json:"match"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &tj); err != nil {
		t.Fatal(err)
	}
	tj.Tables[0].Match = k
	raw, _ = json.MarshalIndent(tj, "", "  ")
	_ = os.WriteFile(p, raw, 0o644)
}

func TestReadLogIgnoresATrailingPartialLine(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "t1"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.events"), []byte(`{"seq":0,"kind":0,"amount":2}`+"\n"+`{"seq":1,"kind":20,"player":1,"amount":1}`+"\n"+`{"seq":2,"ki`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.intents"), []byte(`{"seq":2,"player":0,"choices":[1]}`+"\n"+`{"seq":`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.json"), []byte(`{"table":"t1","match":1,"seed":7,"state":"live"}`), 0o644)
	l, err := readLog(dir, "t1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if l.Seed != 7 || len(l.Events) != 2 || len(l.Intents) != 0 || l.Events[1].Kind != events.DecisionAsk {
		t.Fatalf("log %+v", l)
	}
	if _, err := readLog(dir, "t1", 2); err == nil {
		t.Fatal("missing match read without error")
	}
}

// TestRestartServesAStablePrefixAfterMidBurstTruncation is fix round 1's
// burst-atomicity regression (item 2): a crash or kill can leave a match's
// 3.events file cut inside its final burst, with the owning intent already
// on disk (append writes the intent before the burst it owns). On restart
// Events and ViewAt must succeed and return the surviving prefix — the
// original log's first N events, trimmed to complete bursts — never the
// unreplayable tail.
func TestRestartServesAStablePrefixAfterMidBurstTruncation(t *testing.T) {
	dir := t.TempDir()
	playOneToDisk(t, dir)

	p := filepath.Join(dir, "t1", "1.events")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	origEvents := 0
	lastAsk := -1
	for i, ln := range lines {
		if ln == "" {
			continue
		}
		origEvents++
		var e events.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatal(err)
		}
		if e.Kind == events.DecisionAsk {
			lastAsk = i
		}
	}

	// Cut mid-burst: keep everything up to the first event of the burst the
	// last DecisionAsk opened, dropping the DecisionAsk/GameOver that would
	// have completed it. That leaves a real, mid-burst tail on disk whose
	// owning intent was already written.
	var cut int
	if lastAsk >= 0 && lastAsk+1 < len(lines) && lines[lastAsk+1] != "" {
		cut = lastAsk + 1
	} else {
		// Nothing after the last ask — drop the trailing GameOver line so
		// the final burst is left without its closer.
		cut = len(lines) - 2
		if cut < 0 {
			t.Fatal("fixture too short to truncate mid-burst")
		}
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines[:cut+1], "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	bodies, err := r.Events("t1", 1, 0)
	if err != nil {
		t.Fatalf("Events after truncation: %v", err)
	}
	if len(bodies) == 0 || len(bodies) >= origEvents {
		t.Fatalf("truncated match served %d events (original %d), want a strict non-empty prefix", len(bodies), origEvents)
	}
	// The served prefix is exactly the original's first len(bodies) events,
	// in order — the surviving prefix, not garbage.
	for i, b := range bodies {
		if b.Event.Seq != uint64(i) {
			t.Fatalf("served prefix is not contiguous: body %d has seq %d", i, b.Event.Seq)
		}
	}
	if _, err := r.ViewAt("t1", 1, uint64(len(bodies)-1)); err != nil {
		t.Fatalf("ViewAt(surviving head) after truncation: %v", err)
	}
	if _, err := r.ViewAt("t1", 1, 0); err != nil {
		t.Fatalf("ViewAt(genesis) after truncation: %v", err)
	}
}

// TestSyncLeavesNoTempFiles is fix round 1's fsync-coverage check (item
// 3): with Options.Sync set, a full play to disk must both fsync the temp
// files before their renames (events, intents, every sidecar, tables.json)
// and not leave any *.tmp behind on any path.
func TestSyncLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	o := diskOptions(t, dir)
	o.Sync = true
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	r.Close()

	err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(info.Name(), ".tmp") {
			t.Errorf("left behind: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestArchivedMatchIsServedSafelyUnderConcurrentReaders is fix round 1's
// t.loaded data-race regression (item 1): reading an archived match from
// several goroutines at once, in persist mode, must load it exactly once.
// The first reader upgrades to the write lock and rebuilds the match; the
// others must re-check t.loaded after the upgrade rather than race a bare
// assignment made under only the read lock. (The controller's -race gate is
// what would catch the old bug; this test just makes the load genuinely
// concurrent.)
func TestArchivedMatchIsServedSafelyUnderConcurrentReaders(t *testing.T) {
	dir := t.TempDir()
	playOneToDisk(t, dir)
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches: %+v, %v", ms, err)
	}
	seqs := []uint64{0, uint64(ms[0].Events) / 2, uint64(ms[0].Events) - 1}
	errs := make(chan error, 6*len(seqs)*2)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		for _, seq := range seqs {
			wg.Add(1)
			go func(s uint64) {
				defer wg.Done()
				if _, err := r.ViewAt("t1", 1, s); err != nil {
					errs <- fmt.Errorf("ViewAt(%d): %w", s, err)
				}
				if _, err := r.Events("t1", 1, 0); err != nil {
					errs <- fmt.Errorf("Events: %w", err)
				}
			}(seq)
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
