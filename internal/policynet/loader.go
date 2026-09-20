package policynet

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/traceboard"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// LabelSchemaVersion and LabelRecordType mirror cmd/searchteacher/labels.go's
// writer constants. The record type itself is mirrored below (the writer is
// package main and cannot be imported); a drift between the two shapes is a
// decoder error on the fields that matter, not a silent misread.
const (
	LabelSchemaVersion = 1
	LabelRecordType    = "label-v1"
)

// labelRecord mirrors cmd/searchteacher/labels.go's LabelRecord: the merged
// search-teacher label corpus record, one JSON object per line. Field names
// and JSON tags must match the writer's exactly.
type labelRecord struct {
	RecordType    string            `json:"record_type"`
	SchemaVersion int               `json:"schema_version"`
	Pair          string            `json:"pair"`
	GameIndex     int               `json:"game_index"`
	Seed          uint64            `json:"seed"`
	Sequence      uint64            `json:"decision_sequence"`
	Seat          state.PlayerID    `json:"seat"`
	Kind          decision.Kind     `json:"kind"`
	Turn          int32             `json:"turn"`
	Board         traceboard.Board  `json:"board"`
	View          json.RawMessage   `json:"view"`
	Options       []decision.Option `json:"options"`
	Candidates    []labelCandidate  `json:"candidates"`
	TeacherChoice int               `json:"teacher_choice"`
	BotIndex      int               `json:"bot_index"`
	Margin        float64           `json:"margin"`
	Worlds        int               `json:"worlds"`
	Attempts      int               `json:"attempts"`
	Accepted      int               `json:"accepted"`
	Horizon       int32             `json:"horizon"`
}

// labelCandidate mirrors cmd/searchteacher/labels.go's LabelCandidate.
type labelCandidate struct {
	Choices []int   `json:"choices"`
	Bot     bool    `json:"bot"`
	Value   float64 `json:"value"`
	Worlds  int     `json:"worlds"`
}

// Example is one loaded training example: the encoded state, the encoded
// offered options (parallel to the record's option list, each carrying its
// target), and the record's identifying metadata.
type Example struct {
	Pair      string
	GameIndex int
	Seed      uint64
	Sequence  uint64
	Seat      state.PlayerID
	Kind      decision.Kind
	Turn      int32
	// Margin is the record's margin: the best candidate mean minus the bot's
	// mean over EVERY candidate. TeacherChoice != 0 implies Margin > 0, but
	// not the converse (see the writer's doc).
	Margin        float64
	TeacherChoice int
	BotIndex      int
	State         State
	Options       []Option
}

// Stats counts what Load saw.
type Stats struct {
	// Records is every label record read, skipped ones included.
	Records int
	// Skipped is the records whose candidates list was empty (the writer
	// never emits one, but a consumer must not train on them).
	Skipped int
}

// Load streams a label corpus (plain or gzip-compressed JSONL, detected by
// magic bytes) into decoded, encoded Examples without holding the whole file
// in memory: records are read one at a time and encoded as they arrive.
//
// A record whose record_type or schema_version does not match the writer's
// constants is a hard error (a wrong-shape corpus must not train anything).
// A record whose candidates list is empty is skipped and counted in
// Stats.Skipped.
func Load(path string) ([]Example, Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Stats{}, fmt.Errorf("opening label corpus: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var stream io.Reader = r
	if magic, err := r.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, Stats{}, fmt.Errorf("reading gzipped label corpus: %w", err)
		}
		defer zr.Close()
		stream = zr
	}

	dec := json.NewDecoder(stream)
	var out []Example
	var stats Stats
	for {
		var rec labelRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return nil, stats, fmt.Errorf("label corpus record %d: %w", stats.Records+1, err)
		}
		stats.Records++
		if rec.RecordType != LabelRecordType {
			return nil, stats, fmt.Errorf("label corpus record %d: wrong record_type %q, want %q", stats.Records, rec.RecordType, LabelRecordType)
		}
		if rec.SchemaVersion != LabelSchemaVersion {
			return nil, stats, fmt.Errorf("label corpus record %d: unsupported schema_version %d, want %d", stats.Records, rec.SchemaVersion, LabelSchemaVersion)
		}
		if len(rec.Candidates) == 0 {
			stats.Skipped++
			continue
		}
		var v view.View
		if err := json.Unmarshal(rec.View, &v); err != nil {
			return nil, stats, fmt.Errorf("label corpus record %d: decoding view: %w", stats.Records, err)
		}
		targets := optionTargets(&rec)
		ex := Example{
			Pair:          rec.Pair,
			GameIndex:     rec.GameIndex,
			Seed:          rec.Seed,
			Sequence:      rec.Sequence,
			Seat:          rec.Seat,
			Kind:          rec.Kind,
			Turn:          rec.Turn,
			Margin:        rec.Margin,
			TeacherChoice: rec.TeacherChoice,
			BotIndex:      rec.BotIndex,
			State:         EncodeState(v, rec.Seat),
			Options:       make([]Option, len(rec.Options)),
		}
		for i := range rec.Options {
			ex.Options[i] = EncodeOption(v, rec.Seat, rec.Kind, rec.Options[i], i, len(rec.Options))
			ex.Options[i].Target = targets[i]
		}
		out = append(out, ex)
	}
	return out, stats, nil
}

// optionTargets computes one OptionTarget per offered option: the FIRST
// candidate (in record order) whose answer contains the option supplies the
// value; the teacher's chosen candidate marks preferred. Deterministic —
// record order, never a map.
func optionTargets(rec *labelRecord) []OptionTarget {
	out := make([]OptionTarget, len(rec.Options))
	for _, c := range rec.Candidates {
		for _, j := range c.Choices {
			if j < 0 || j >= len(out) {
				continue
			}
			if !out[j].Labelled {
				out[j] = OptionTarget{Labelled: true, Value: c.Value}
			}
		}
	}
	if rec.TeacherChoice >= 0 && rec.TeacherChoice < len(rec.Candidates) {
		for _, j := range rec.Candidates[rec.TeacherChoice].Choices {
			if j >= 0 && j < len(out) {
				out[j].Preferred = true
			}
		}
	}
	return out
}
