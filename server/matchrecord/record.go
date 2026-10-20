// Package matchrecord is the portable record of one played game: the
// replay input (the match's genesis configuration and its events.Log) plus
// who produced it, written as one gzip-compressed JSON file per game,
// `<id>.match.json.gz` (match-reviewer design D1-D3).
//
// A game is fully determined by (Config, Log), never by the Log alone
// (engine/replay's package doc), so a record carries the Config's
// replay-input subset in the shape host.FeedbackMatch already uses for a
// feedback snapshot: seed, names, the deck CONTENTS by card name,
// sideboards, format, life, commanders, mulligans, the starting-player
// chooser and the two match-mode bits (name universe, paid-mana census).
// It deliberately carries neither the token scripts nor the name-universe
// label list (D2): Forge token scripts are GPL-3.0 and the label list is
// ~24k entries. The record names the corpus pin (Producer.ForgeRef)
// instead, and Load re-derives both from the registry it is handed, exactly
// the way a committed feedback fixture replays. A record from another pin
// therefore replays as a DIVERGED, the existing honest failure.
//
// A record holds every hidden card by construction (the log is the
// omniscient history), so a record of a real game is an unpublished
// artifact: never commit one. Tests generate theirs in a temp dir.
//
// Layering contract: this is a server-tier leaf. Its only gorge imports are
// engine packages (cards, events, replay, rules); host, bots and testkit
// are forbidden, so a bench driver, cmd/repro and the review server can all
// depend on it without a cycle. Nothing here reads a clock or ranges over a
// map whose order can reach output.
package matchrecord

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
)

// FormatV1 is the record's format tag. Load refuses any other.
const FormatV1 = "gorge-match-1"

// Ext is the file suffix a record is written under.
const Ext = ".match.json.gz"

// Record is one game's match record.
type Record struct {
	// Format is FormatV1.
	Format string `json:"format"`
	// Producer says which tool and build wrote the record. It is provenance,
	// not replay input.
	Producer Producer `json:"producer"`
	// Match is the replay-input subset of the game's rules.Config.
	Match Match `json:"match"`
	// Log is the game's event log: seed, events and intents, the exact
	// replay input beside Match.
	Log *events.Log `json:"log"`
	// Head is the chain head over every logged event, what Verify's replay
	// must reproduce.
	Head string `json:"head"`
	// Chance is the chance transcript of a kernel-sourced game (design
	// §2.3, D4). Phase 1 never writes it; Verify refuses a record that
	// carries one, since the ordinary replay implements Config-only
	// randomness.
	Chance []rules.ChanceDraw `json:"chance,omitempty"`
	// Explain is the per-intent decision sidecar (design §3). It is never
	// replay input, never hashed, and empty until the explain emitters land
	// (ticket T3); its entry shape belongs to that ticket.
	Explain []json.RawMessage `json:"explain,omitempty"`
}

// Producer is a record's provenance.
type Producer struct {
	// Tool names the driver that played the game (botbench, dzgorge, ...).
	Tool string `json:"tool,omitempty"`
	// GorgeCommit is the gorge build that played it (see GorgeCommit).
	GorgeCommit string `json:"gorge_commit,omitempty"`
	// ForgeRef is the card-corpus pin the game was played against (see
	// ForgeRef): the corpus Load must be handed for the record to replay.
	ForgeRef string `json:"forge_ref,omitempty"`
	// Policies names the policy at each seat, in seat order.
	Policies []string `json:"policies,omitempty"`
	// RunID groups the records of one run.
	RunID string `json:"run_id,omitempty"`
}

// New builds the record of the game cfg configured and l logged. The
// record references l rather than copying it: it is meant to be written
// (Write) before l's owner touches the log again.
func New(cfg rules.Config, l *events.Log, p Producer) (*Record, error) {
	if l == nil {
		return nil, fmt.Errorf("matchrecord: nil log")
	}
	if l.NoHash {
		return nil, fmt.Errorf("matchrecord: the log keeps no hash chain (NoHash), so no replay could verify it")
	}
	m, err := MatchOf(cfg)
	if err != nil {
		return nil, err
	}
	m.Seed = l.Seed
	return &Record{Format: FormatV1, Producer: p, Match: m, Log: l, Head: l.Head()}, nil
}

// Write writes r to path as gzip-compressed JSON. The file is written
// beside its final name and renamed into place, so a reader never sees a
// partial record.
func Write(path string, r *Record) (err error) {
	if r == nil || r.Log == nil {
		return fmt.Errorf("matchrecord: write %s: record has no log", path)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".matchrecord-*")
	if err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	zw, err := gzip.NewWriterLevel(f, gzip.DefaultCompression)
	if err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	if err = json.NewEncoder(zw).Encode(r); err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	if err = zw.Close(); err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	// CreateTemp makes the file owner-only; a record is an ordinary output
	// file of its run, readable the way the run's other outputs are.
	if err = f.Chmod(0o644); err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	if err = os.Rename(tmp, path); err != nil {
		return fmt.Errorf("matchrecord: write %s: %w", path, err)
	}
	return nil
}

// Read decodes the record at path without building a Config: the
// provenance and log alone, for a caller that only lists or inspects.
func Read(path string) (*Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("matchrecord: %w", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("matchrecord: %s: %w", path, err)
	}
	defer zr.Close()
	var r Record
	if err := json.NewDecoder(zr).Decode(&r); err != nil {
		return nil, fmt.Errorf("matchrecord: %s: %w", path, err)
	}
	if r.Format != FormatV1 {
		return nil, fmt.Errorf("matchrecord: %s: format %q, want %q", path, r.Format, FormatV1)
	}
	if r.Log == nil {
		return nil, fmt.Errorf("matchrecord: %s: record has no log", path)
	}
	return &r, nil
}

// Load reads the record at path and rebuilds the rules.Config its replay
// needs, resolving the recorded card names, the token table and (when the
// match ran with one) the name universe through reg — the corpus at the
// record's Producer.ForgeRef, for the replay to reproduce it.
func Load(path string, reg *cards.Registry) (*Record, rules.Config, error) {
	r, err := Read(path)
	if err != nil {
		return nil, rules.Config{}, err
	}
	cfg, err := r.Match.Config(reg, reg.Tokens)
	if err != nil {
		return r, rules.Config{}, fmt.Errorf("matchrecord: %s: %w", path, err)
	}
	return r, cfg, nil
}

// Verify replays r against cfg and checks that the replay reproduces the
// recorded head. A *replay.Divergence stays reachable through errors.As,
// so a caller can name the first diverging event.
func Verify(r *Record, cfg rules.Config) error {
	if r == nil || r.Log == nil {
		return fmt.Errorf("matchrecord: verify: record has no log")
	}
	if len(r.Chance) != 0 {
		return fmt.Errorf("matchrecord: verify: the record carries a chance transcript, which the ordinary replay does not take")
	}
	if want := r.Log.HeadAt(len(r.Log.Events)); want != r.Head {
		return fmt.Errorf("matchrecord: verify: recorded head %q does not match the log's own event chain %q", r.Head, want)
	}
	e, err := replay.Replay(r.Log, cfg)
	if err != nil {
		return fmt.Errorf("matchrecord: verify: %w", err)
	}
	if got := e.L.Head(); got != r.Head {
		return fmt.Errorf("matchrecord: verify: replayed head %s, recorded %s", got, r.Head)
	}
	return nil
}
