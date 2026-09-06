package host

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
)

// sidecar is <k>.json: everything about a match except its events and
// intents. Rewritten whole at start and at end; never carries a timestamp
// (PL-11), so two runs of one configuration write identical files.
type sidecar struct {
	Table     string              `json:"table"`
	Match     int                 `json:"match"`
	Seed      uint64              `json:"seed"`
	Seats     []protocol.SeatInfo `json:"seats"`
	Names     []string            `json:"names"`
	Decks     []string            `json:"decks"`
	Spectator string              `json:"spectator"`
	State     string              `json:"state"`
	Result    string              `json:"result,omitempty"`
	Winner    *uint8              `json:"winner"`
	Head      string              `json:"head,omitempty"`
	Events    int                 `json:"events"`
	Turns     int32               `json:"turns"`
	Reason    string              `json:"reason,omitempty"`
	// Mulligans is the London-mulligan allowance the match's rules.Config ran
	// with, persisted so a restart rebuilds a replay Config that reproduces
	// the match (R-8.4; host/viewat.go rebuilds its Config from this sidecar
	// alone, not from the live TableConfig). omitempty keeps both an old
	// sidecar that predates the field and a no-mulligan match loading as
	// exactly 0 — the value those matches played with — so nothing is
	// migrated or versioned (R-E5-2).
	Mulligans int `json:"mulligans,omitempty"`
}

func (sc sidecar) info() protocol.MatchInfo {
	return protocol.MatchInfo{Table: sc.Table, Match: sc.Match, Seed: sc.Seed, Seats: sc.Seats, State: sc.State,
		Result: sc.Result, Winner: sc.Winner, Head: sc.Head, Events: sc.Events, Turns: sc.Turns}
}

// matchFiles are a live match's append-only logs.
type matchFiles struct {
	events, intents *os.File
	sync            bool
}

func tableDir(dir string, t TableID) string { return filepath.Join(dir, string(t)) }

func matchPath(dir string, t TableID, k int, ext string) string {
	return filepath.Join(tableDir(dir, t), strconv.Itoa(k)+ext)
}

// openMatchFiles creates (truncating) a match's two logs.
func openMatchFiles(dir string, t TableID, k int, sync bool) (*matchFiles, error) {
	if err := os.MkdirAll(tableDir(dir, t), 0o755); err != nil {
		return nil, err
	}
	ev, err := os.Create(matchPath(dir, t, k, ".events"))
	if err != nil {
		return nil, err
	}
	in, err := os.Create(matchPath(dir, t, k, ".intents"))
	if err != nil {
		ev.Close()
		return nil, err
	}
	return &matchFiles{events: ev, intents: in, sync: sync}, nil
}

// append writes one burst: the new events and the intent that produced
// them (nil for genesis), one JSON object per line, then fsyncs when
// configured (PL-13, opts.Sync).
//
// Burst atomicity (fix round 1): the intent that owns a burst is written
// BEFORE the burst's events. The two live in separate files and can never
// be committed atomically, and this ordering is the safe side: it guarantees
// the events stream is never longer than the intents that account for it.
// The one leftover a crash or kill can leave is an orphan intent at the
// tail whose events never made it, which read-time reconcile (reconcileLog)
// trims back so a restart always serves a consistent prefix.
func (f *matchFiles) append(evs []events.Event, in *decision.Intent) error {
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		if _, err := f.intents.Write(append(b, '\n')); err != nil {
			return err
		}
		if f.sync {
			if err := f.intents.Sync(); err != nil {
				return err
			}
		}
	}
	w := bufio.NewWriter(f.events)
	for _, e := range evs {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if f.sync {
		if err := f.events.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (f *matchFiles) close() {
	if f == nil {
		return
	}
	f.events.Close()
	f.intents.Close()
}

// writeSidecar writes <k>.json atomically (temp file + rename). When sync
// is set the temp is fsynced before the rename and the parent directory
// after it (fix round 1, fsync coverage) so the rename, not just the file's
// contents, is durable. The temp file is removed on every error path and
// consumed by the rename, so no *.tmp survives (writeFileAtomic).
func writeSidecar(dir string, sc sidecar, sync bool) error {
	if err := os.MkdirAll(tableDir(dir, TableID(sc.Table)), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(matchPath(dir, TableID(sc.Table), sc.Match, ".json"), append(b, '\n'), sync)
}

// writeFileAtomic writes b to path via a temp file in the same directory:
// write + fsync the temp (contents durable before the rename exposes
// them), rename it over path, then fsync the parent directory (the rename
// itself durable). The temp is removed on every error path and consumed by
// the rename on success, so no *.tmp is ever left behind.
//
// The directory fsync is the one step some filesystems reject (e.g. EINVAL
// on Windows); it is best-effort, because the temp already sync'd and
// renamed is the durable part and failing the dir sync would otherwise
// break writes to otherwise-supported paths.
func writeFileAtomic(path string, b []byte, sync bool) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	if werr == nil && sync {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	if sync {
		if d, err := os.Open(filepath.Dir(path)); err == nil {
			d.Sync()
			d.Close()
		}
	}
	return nil
}

func readSidecar(dir string, t TableID, k int) (sidecar, error) {
	var sc sidecar
	raw, err := os.ReadFile(matchPath(dir, t, k, ".json"))
	if err != nil {
		return sc, err
	}
	if err := json.Unmarshal(raw, &sc); err != nil {
		return sc, fmt.Errorf("host: %s/%d.json: %w", t, k, err)
	}
	return sc, nil
}

// readSidecars lists a table's matches from its sidecars, ascending by k.
func readSidecars(dir string, t TableID) ([]sidecar, error) {
	entries, err := os.ReadDir(tableDir(dir, t))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []sidecar
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		k, err := strconv.Atoi(name[:len(name)-len(".json")])
		if err != nil {
			continue
		}
		sc, err := readSidecar(dir, t, k)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Match < out[j].Match })
	return out, nil
}

// readLog rebuilds a match's events.Log from its files. A trailing partial
// line (a crash mid-write) is ignored; anything malformed before it is an
// error. The Seed comes from the sidecar. The two streams are reconciled to
// a consistent prefix (fix round 1) before returning, so Events/ViewAt of a
// match whose write was cut off never serve a log that cannot replay.
func readLog(dir string, t TableID, k int) (*events.Log, error) {
	sc, err := readSidecar(dir, t, k)
	if err != nil {
		return nil, err
	}
	l := events.NewLog(sc.Seed)
	if err := readLines(matchPath(dir, t, k, ".events"), func(b []byte) error {
		var e events.Event
		if err := json.Unmarshal(b, &e); err != nil {
			return err
		}
		l.Append(e)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := readLines(matchPath(dir, t, k, ".intents"), func(b []byte) error {
		var in decision.Intent
		if err := json.Unmarshal(b, &in); err != nil {
			return err
		}
		l.Intents = append(l.Intents, in)
		return nil
	}); err != nil {
		return nil, err
	}
	reconcileLog(l)
	return l, nil
}

// reconcileLog trims a log read from disk to the consistent prefix a replay
// can rebuild (fix round 1, burst atomicity). A burst's events and its
// owning intent live in two separate files and can never be committed
// atomically, so a crash or kill mid-burst can leave either stream ahead of
// the other: the events file can be cut inside its last burst (the
// DecisionAsk or GameOver that would complete it never written, so the tail
// is not a complete burst), and — because append writes the intent before
// its burst — the intents file can hold an orphan intent whose events never
// made it. boundsOf marks every complete burst; each non-genesis burst needs
// exactly one intent, so a log with C complete bursts is trustworthy only up
// to C-1 of its intents and the C bursts those account for, and neither
// stream may run past the other. Everything past that is trimmed, so
// Events and ViewAt always serve a prefix that replays cleanly.
func reconcileLog(l *events.Log) {
	bounds := boundsOf(l.Events)
	if len(bounds) == 0 {
		// No complete burst — not even the genesis burst (boundary 0) a
		// wiped match would still have. Nothing here can replay, so drop
		// both streams rather than serve events' untrustworthy tail.
		l.Events, l.Intents = nil, nil
		return
	}
	// C complete bursts want C-1 intents (genesis needs none). Clamp to
	// whatever the intents file actually has, so the events stream never
	// runs past what the intents can drive.
	need := len(bounds) - 1
	if n := len(l.Intents); n < need {
		need = n
	}
	// bounds[need] is one past burst `need`, the last burst still owned by
	// an intent we hold (for need > 0) or the genesis burst (for need == 0).
	// Everything before that boundary replays; everything after is the tail
	// a crash cut off and must not be served.
	l.Events = l.Events[:bounds[need]]
	if len(l.Intents) > need {
		l.Intents = l.Intents[:need]
	}
}

// readLines calls fn per complete, newline-terminated record. A file that
// ends with '\n' splits into a trailing empty element; one cut off
// mid-write ends with a partial record. Either way the final element is
// not a complete record and is dropped.
func readLines(path string, fn func([]byte) error) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(raw, []byte{'\n'})
	for _, line := range lines[:len(lines)-1] {
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return fmt.Errorf("host: %s: %w", path, err)
		}
	}
	return nil
}

// tablesFile is tables.json.
type tablesFile struct {
	Tables []tableRecord `json:"tables"`
}

type tableRecord struct {
	Config TableConfig `json:"config"`
	Match  int         `json:"match"`
}

// save writes tables.json (sorted by id) when persistence is on. It takes
// the registry lock itself; callers that already hold r.mu use saveLocked.
// fsync coverage (fix round 1): the temp file is synced before the rename
// and the parent directory after it (writeFileAtomic), the same guarantees
// as the sidecar and the events files.
func (r *Registry) save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

// saveLocked is the body of save with r.mu already held (AddTable, run and
// archive call it that way); save is saveLocked plus the lock it takes
// itself. Persisting is no-op in memory mode.
func (r *Registry) saveLocked() error {
	if r.opts.Dir == "" {
		return nil
	}
	var tf tablesFile
	ids := make([]string, 0, len(r.tables))
	for id := range r.tables {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := r.tables[TableID(id)]
		t.mu.RLock()
		tf.Tables = append(tf.Tables, tableRecord{Config: t.cfg, Match: t.k})
		t.mu.RUnlock()
	}
	if err := os.MkdirAll(r.opts.Dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(r.opts.Dir, "tables.json"), append(b, '\n'), r.opts.Sync)
}
