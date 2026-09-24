package policynet

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// The oracle state/outcome dump (ticket pn17): a JSONL corpus of encoded
// STATES with game outcomes and NO labelled options. It is the value-only
// training path's input (cmd/policytrain -value-corpus): one record per
// observed board state, carrying the redacted seat view the encoders read,
// so the same record re-encodes under every feature set (v1, mz, and — for
// measurement only — the diagnostic mz-opphand / mz-oracle with the extras'
// Diag* fields).
//
// The record shape mirrors the label corpus's conventions (record_type /
// schema_version envelope, gzip-by-magic, the LabelExtras "extras" object
// re-used unchanged for the hidden information) but is a DIFFERENT record
// type: a label record trains the policy, a state record trains the value
// head only. The record type string and schema version are the writer
// contract; a future dump generator (cmd/searchteacher or the oracle runs)
// mirrors them exactly the way cmd/searchteacher/labels.go mirrors
// LabelRecordType.
const (
	StateDumpRecordType       = "state-v1"
	StateDumpSchemaVersion    = 1
	acceptedStateDumpVersions = 1
)

// StateRecord is one state/outcome dump record, one JSON object per line.
// The JSON tags are the writer contract (mirrored by the producer; the
// loader reads exactly these names).
type StateRecord struct {
	RecordType    string         `json:"record_type"`
	SchemaVersion int            `json:"schema_version"`
	Pair          string         `json:"pair"`
	GameIndex     int            `json:"game_index"`
	Seed          uint64         `json:"seed"`
	Sequence      uint64         `json:"sequence"`
	Seat          state.PlayerID `json:"seat"`
	Turn          int32          `json:"turn"`
	// Outcome is the deciding seat's game result, 1 win / 0.5 draw / 0 loss,
	// meaningful only when OutcomeKnown (a stalled or unfinished game records
	// outcome_known false and trains nothing).
	Outcome      float64         `json:"outcome"`
	OutcomeKnown bool            `json:"outcome_known"`
	View         json.RawMessage `json:"view"`
	// Extras is the label corpus's pn12 extras object re-used unchanged: only
	// its Diag* fields matter here, and only for the diagnostic feature sets.
	Extras *LabelExtras `json:"extras"`
}

// StateExample is one loaded state/outcome record: the encoded state under
// the requested feature set, the identifying metadata the value trainer's
// seed-block split reads, and the outcome target.
type StateExample struct {
	Pair      string
	GameIndex int
	Seed      uint64
	Sequence  uint64
	Seat      state.PlayerID
	Turn      int32
	Outcome   float64
	State     State
}

// StateDumpStats counts what LoadStateOutcome saw.
type StateDumpStats struct {
	// Records is every record read, skipped ones included.
	Records int
	// Loaded is the records encoded into a StateExample.
	Loaded int
	// Skipped is the records with outcome_known false (a stalled or
	// unfinished game trains nothing).
	Skipped int
}

// LoadStateOutcome streams a state/outcome dump (plain or gzip-compressed
// JSONL, detected by magic bytes) into StateExamples encoded under feature
// set fs. A diagnostic fs REQUIRES every record to carry extras: without
// them the diagnostic tokens would silently be absent and the encoding
// would deceptively equal the redacted one, so the loader fails closed.
func LoadStateOutcome(path string, fs FeatureSet) ([]StateExample, StateDumpStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, StateDumpStats{}, fmt.Errorf("opening state dump: %w", err)
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var stream io.Reader = r
	if magic, err := r.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, StateDumpStats{}, fmt.Errorf("reading gzipped state dump: %w", err)
		}
		defer zr.Close()
		stream = zr
	}
	dec := json.NewDecoder(stream)
	var out []StateExample
	var stats StateDumpStats
	for {
		var rec StateRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return nil, stats, fmt.Errorf("state dump record %d: %w", stats.Records+1, err)
		}
		stats.Records++
		if rec.RecordType != StateDumpRecordType {
			return nil, stats, fmt.Errorf("state dump record %d: wrong record_type %q, want %q", stats.Records, rec.RecordType, StateDumpRecordType)
		}
		if rec.SchemaVersion != acceptedStateDumpVersions {
			return nil, stats, fmt.Errorf("state dump record %d: unsupported schema_version %d, want %d", stats.Records, rec.SchemaVersion, acceptedStateDumpVersions)
		}
		if len(rec.View) == 0 || bytes.Equal(bytes.TrimSpace(rec.View), []byte("null")) {
			return nil, stats, fmt.Errorf("state dump record %d: missing view", stats.Records)
		}
		var v view.View
		if err := json.Unmarshal(rec.View, &v); err != nil {
			return nil, stats, fmt.Errorf("state dump record %d: decoding view: %w", stats.Records, err)
		}
		var diag *Diag
		if fs.Diagnostic() {
			if rec.Extras == nil {
				return nil, stats, fmt.Errorf("state dump record %d: diagnostic feature set %s on a record with no extras — the diagnostic tokens would silently be absent", stats.Records, fs)
			}
			diag = &Diag{OppHand: rec.Extras.DiagOppHand, OppLibraryTop: rec.Extras.DiagOppLibraryTop, OwnLibraryTop: rec.Extras.DiagOwnLibraryTop}
		}
		if !rec.OutcomeKnown {
			stats.Skipped++
			continue
		}
		out = append(out, StateExample{
			Pair: rec.Pair, GameIndex: rec.GameIndex, Seed: rec.Seed, Sequence: rec.Sequence,
			Seat: rec.Seat, Turn: rec.Turn,
			Outcome: rec.Outcome,
			State:   EncodeStateWith(fs, v, rec.Seat, diag),
		})
		stats.Loaded++
	}
	return out, stats, nil
}
