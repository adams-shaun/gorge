package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/traceboard"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

const labelSchemaVersion = 1

// LabelRecord is one search-teacher decision the teacher COVERED (it scored
// every candidate on worlds and produced values): the deciding seat's board
// and view snapshots, the full offered options, the evaluated candidates and
// the teacher's verdict. Written by -labels, one JSON object per line,
// sorted by pair, game index, decision sequence — so a corpus built with any
// -workers value is byte-identical for fixed flags. Timing is deliberately
// absent (wall-clock has no place in a training corpus; the -out
// GameRecords carry it).
type LabelRecord struct {
	RecordType    string         `json:"record_type"`
	SchemaVersion int            `json:"schema_version"`
	Pair          string         `json:"pair"`
	GameIndex     int            `json:"game_index"`
	Seed          uint64         `json:"seed"`
	Sequence      uint64         `json:"decision_sequence"`
	Seat          state.PlayerID `json:"seat"`
	Kind          decision.Kind  `json:"kind"`
	Turn          int32          `json:"turn"`
	// Board is the deciding seat's redacted board snapshot in botbench's
	// decision-trace board schema (internal/traceboard, board_schema_version
	// 1) — the SAME type the trace records decode into.
	Board traceboard.Board `json:"board"`
	// View is the deciding seat's own view.Project JSON with Decision set to
	// nil (engine continuation state is never serialized; the offered
	// options live in Options below). Redacted by view's CR 400.2 rules: the
	// opponent's hand is null and no opponent hand card name appears.
	View json.RawMessage `json:"view"`
	// Options is the full offered decision.Option list (every wire field:
	// Obj, Player, Attacker, AltCostIndex, Mode, Amount, Ability, plus
	// Label/Required/Group/SVar/Cost — everything a seat itself was offered).
	Options []decision.Option `json:"options"`
	// Candidates is the evaluated candidate set in teacher order: index 0 is
	// always the bot's answer.
	Candidates []LabelCandidate `json:"candidates"`
	// TeacherChoice is the winning candidate index (0 = the bot's answer was
	// kept); BotIndex is the candidate index carrying the bot's answer (0 by
	// construction). Margin is the best candidate mean minus the bot's mean
	// (0 when the teacher kept the bot).
	TeacherChoice int     `json:"teacher_choice"`
	BotIndex      int     `json:"bot_index"`
	Margin        float64 `json:"margin"`
	// Worlds is K, the number of sampled worlds every candidate was rolled on
	// (1 in -oracle mode); Attempts/Accepted are this decision's sampler
	// proposal attempts and acceptances (0 in -oracle mode); Horizon is the
	// rollout horizon in engine turns (0 = game end).
	Worlds   int   `json:"worlds"`
	Attempts int   `json:"attempts"`
	Accepted int   `json:"accepted"`
	Horizon  int32 `json:"horizon"`
}

// LabelCandidate is one evaluated candidate answer: its option set (the
// option indices it selects in Options), its mean rollout value from the
// deciding seat's point of view, the number of worlds it was evaluated on,
// and whether it is the bot's answer.
type LabelCandidate struct {
	Choices []int   `json:"choices"`
	Bot     bool    `json:"bot"`
	Value   float64 `json:"value"`
	Worlds  int     `json:"worlds"`
}

// seatView projects the deciding seat's own view with the decision omitted
// (the internal/searchprobe Capture pattern: never serialize engine
// continuation state).
func seatView(e *rules.Engine, d *decision.Decision) (json.RawMessage, error) {
	v := view.Project(e.G, e, d.Player, d)
	v.Decision = nil
	v.Round = view.RoundOf(e.G, e.L.Events)
	return json.Marshal(v)
}

// checkLabelsDestination fails fast on a -labels path that cannot be
// published, before any game is played.
func checkLabelsDestination(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("label corpus parent: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("label corpus parent %q is not a directory", parent)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("label corpus destination %q already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking label corpus destination: %w", err)
	}
	return nil
}

// collectLabels flattens the games' label records in the published order:
// pair (lexicographic), game index, decision sequence. Each game is played
// by one goroutine, so its records are already in sequence order; the sort
// is stable belt-and-braces so the corpus contract does not depend on it.
func collectLabels(all []GameRecord) []LabelRecord {
	var out []LabelRecord
	for _, r := range all {
		out = append(out, r.Labels...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Pair != b.Pair {
			return a.Pair < b.Pair
		}
		if a.GameIndex != b.GameIndex {
			return a.GameIndex < b.GameIndex
		}
		return a.Sequence < b.Sequence
	})
	return out
}

// writeLabels publishes the corpus to a NEW file atomically: every record is
// encoded into a sibling temporary file, synced, closed, and then linked into
// place without ever overwriting an existing destination — the same pattern
// botbench's decision trace uses (publishTraceNoReplace there).
func writeLabels(path string, records []LabelRecord) (err error) {
	if path == "" {
		return nil
	}
	if err := checkLabelsDestination(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating label corpus temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	enc := json.NewEncoder(tmp)
	for i := range records {
		r := records[i]
		if r.RecordType != "label-v1" || r.SchemaVersion != labelSchemaVersion {
			return fmt.Errorf("invalid label record type %q / schema %d", r.RecordType, r.SchemaVersion)
		}
		if r.Board.SchemaVersion != traceboard.SchemaVersion {
			return fmt.Errorf("label board schema version %d is unsupported", r.Board.SchemaVersion)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("writing label corpus: %w", err)
		}
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("syncing label corpus: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing label corpus: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("label corpus destination %q appeared during run", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("rechecking label corpus destination: %w", err)
	}
	if err := os.Link(tmpName, path); err != nil {
		return fmt.Errorf("publishing label corpus: %w", err)
	}
	if err := os.Remove(tmpName); err != nil {
		// Publication is all-or-nothing: if retiring the temporary name
		// fails, remove the published name and surface the cleanup error.
		if rollbackErr := os.Remove(path); rollbackErr != nil {
			return fmt.Errorf("removing published label corpus after temporary cleanup failed: %v (cleanup error: %w)", rollbackErr, err)
		}
		return fmt.Errorf("publishing label corpus: %w", err)
	}
	return nil
}
