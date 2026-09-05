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
// configured (PL-13).
func (f *matchFiles) append(evs []events.Event, in *decision.Intent) error {
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
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		if _, err := f.intents.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	if f.sync {
		if err := f.events.Sync(); err != nil {
			return err
		}
		if err := f.intents.Sync(); err != nil {
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

// writeSidecar writes <k>.json atomically (temp file + rename).
func writeSidecar(dir string, sc sidecar) error {
	if err := os.MkdirAll(tableDir(dir, TableID(sc.Table)), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	p := matchPath(dir, TableID(sc.Table), sc.Match, ".json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
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
// error. The Seed comes from the sidecar.
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
	return l, nil
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

// save writes tables.json (sorted by id) when persistence is on. Called
// with r.mu held.
func (r *Registry) save() error {
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
	p := filepath.Join(r.opts.Dir, "tables.json")
	if err := os.WriteFile(p+".tmp", append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}
