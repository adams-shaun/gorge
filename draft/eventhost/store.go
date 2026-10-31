package eventhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/deck"
)

// meta is one stored event's header, the <dir>/<id>/event.json file. It
// carries everything a run needs before (and without) its log: the drafted
// set, the seed, the four seats and when the event was created (the
// lobby-timeout window; a meta written before the field existed resumes
// with a fresh window). A seat's token is stored only as its sha256; the
// clear token is returned once by CreateEvent and never written.
type meta struct {
	ID        string      `json:"id"`
	Set       string      `json:"set"`
	Seed      uint64      `json:"seed"`
	Seats     [4]metaSeat `json:"seats"`
	CreatedAt time.Time   `json:"created_at,omitempty"`
	// BestOf is the event's series format: 1 (Bo1, the default, stored
	// omitted) or 3 (Bo3). Stored so a restart replays the same
	// format; the log's SeriesStarted/GameStarted entries re-derive the
	// game state either way.
	BestOf int `json:"best_of,omitempty"`
	// PickTimer is the event's pick-timer mode: "off", "standard"
	// or "relaxed". Stored so a restart re-arms the same deadlines; a
	// meta written before the field existed reads back as "" (= off).
	PickTimer string `json:"pick_timer,omitempty"`
}

// metaSeat is one stored seat. Kind is "human" or "bot"; Name is the human
// display name (empty until it joins), Policy the bot's game policy
// (normalised), Drafter the pickbot persona (humans get "rating-colour",
// their suggestion builder), and TokenSHA256 the hex sha256 of the human
// seat's invite token.
type metaSeat struct {
	Kind        string `json:"kind"`
	Name        string `json:"name,omitempty"`
	Policy      string `json:"policy,omitempty"`
	Drafter     string `json:"drafter,omitempty"`
	TokenSHA256 string `json:"token_sha256,omitempty"`
}

// memEvent is one in-memory event of a memory-mode store.
type memEvent struct {
	m     meta
	log   []event.Entry
	decks map[[2]int]deck.File // key {seat, rev}
}

// Store persists draft events under a directory, one subdirectory per
// event: <dir>/<id>/event.json (the meta) and <dir>/<id>/log.jsonl (the
// append-only event log). dir "" keeps everything in memory (tests).
// Every method is safe for concurrent use.
type Store struct {
	dir string
	mu  sync.Mutex
	mem map[string]*memEvent
}

// idShape is the event-id shape: "e" plus a decimal number.
var idShape = regexp.MustCompile(`^e[0-9]+$`)

// OpenStore opens (creating dir when given) the event store. An empty dir
// keeps everything in memory.
func OpenStore(dir string) (*Store, error) {
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("eventhost: open store %s: %w", dir, err)
		}
	}
	return &Store{dir: dir, mem: map[string]*memEvent{}}, nil
}

// ids lists the stored event ids, sorted by numeric suffix (e2 < e10).
func (s *Store) ids() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		out := make([]string, 0, len(s.mem))
		for id := range s.mem {
			out = append(out, id)
		}
		sort.Slice(out, func(i, j int) bool { return idNum(out[i]) < idNum(out[j]) })
		return out, nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("eventhost: store %s: %w", s.dir, err)
	}
	var out []string
	for _, e := range entries {
		id := e.Name()
		if !idShape.MatchString(id) {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.dir, id, "event.json")); err != nil {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return idNum(out[i]) < idNum(out[j]) })
	return out, nil
}

// idNum is the numeric suffix of an id, 0 when the id has none.
func idNum(id string) int {
	n, err := strconv.Atoi(id[1:])
	if err != nil {
		return 0
	}
	return n
}

// newID mints the next free event id "e<N>", N = 1 + the maximum over the
// stored ids and the live ids. live reports whether an id is in use by a
// run the store does not yet know about; every live id was itself minted
// from this sequence, so probing e1..e(maxStored+1) finds them all.
func (s *Store) newID(live func(string) bool) (string, error) {
	stored, err := s.ids()
	if err != nil {
		return "", err
	}
	max := 0
	for _, id := range stored {
		if n := idNum(id); n > max {
			max = n
		}
	}
	for n := 1; n <= max+1; n++ {
		if live != nil && live(fmt.Sprintf("e%d", n)) && n > max {
			max = n
		}
	}
	return fmt.Sprintf("e%d", max+1), nil
}

// saveMeta writes the event's meta to <dir>/<id>/event.json, atomically
// (temp file, fsync, rename; never a *.tmp left behind).
func (s *Store) saveMeta(m meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e := s.mem[m.ID]
		if e == nil {
			e = &memEvent{}
			s.mem[m.ID] = e
		}
		e.m = m
		return nil
	}
	d := filepath.Join(s.dir, m.ID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return fmt.Errorf("eventhost: store %s: %w", s.dir, err)
	}
	b, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return fmt.Errorf("eventhost: marshal meta %s: %w", m.ID, err)
	}
	b = append(b, '\n')
	if err := writeFileAtomic(filepath.Join(d, "event.json"), b); err != nil {
		return fmt.Errorf("eventhost: store %s: %w", s.dir, err)
	}
	return nil
}

// loadMeta reads one stored event's meta.
func (s *Store) loadMeta(id string) (meta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e, ok := s.mem[id]
		if !ok {
			return meta{}, fmt.Errorf("eventhost: no stored event %s", id)
		}
		return e.m, nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, id, "event.json"))
	if err != nil {
		return meta{}, fmt.Errorf("eventhost: read meta %s: %w", id, err)
	}
	var m meta
	if err := json.Unmarshal(b, &m); err != nil {
		return meta{}, fmt.Errorf("eventhost: parse meta %s: %w", id, err)
	}
	return m, nil
}

// appendLog appends one entry to <dir>/<id>/log.jsonl as a JSON line and
// fsyncs it, so a committed pick survives a crash.
func (s *Store) appendLog(id string, en event.Entry) error {
	b, err := json.Marshal(&en)
	if err != nil {
		return fmt.Errorf("eventhost: marshal entry: %w", err)
	}
	b = append(b, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e := s.mem[id]
		if e == nil {
			e = &memEvent{}
			s.mem[id] = e
		}
		e.log = append(e.log, en)
		return nil
	}
	f, err := os.OpenFile(filepath.Join(s.dir, id, "log.jsonl"),
		os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("eventhost: open log %s: %w", id, err)
	}
	_, werr := f.Write(b)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Errorf("eventhost: append log %s: %w", id, werr)
	}
	return nil
}

// readLog reads one event's log back. A final line without its newline —
// a crash mid-append — is dropped here AND truncated off the file, so the
// torn tail cannot replay and cannot come back.
func (s *Store) readLog(id string) ([]event.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e, ok := s.mem[id]
		if !ok {
			return nil, nil
		}
		return append([]event.Entry(nil), e.log...), nil
	}
	path := filepath.Join(s.dir, id, "log.jsonl")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("eventhost: read log %s: %w", id, err)
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		keep := 0
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			keep = i + 1
		}
		if err := os.Truncate(path, int64(keep)); err != nil {
			return nil, fmt.Errorf("eventhost: truncate torn log tail %s: %w", id, err)
		}
		data = data[:keep]
	}
	entries, err := event.ParseLog(data)
	if err != nil {
		return nil, fmt.Errorf("eventhost: parse log %s: %w", id, err)
	}
	return entries, nil
}

// writeFileAtomic writes b to path via a temp file in the same directory:
// write + fsync the temp, rename it over path, then best-effort fsync the
// parent directory. The temp is removed on every error path and consumed
// by the rename on success.
func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	if werr == nil {
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
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
