package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/traceboard"
	"github.com/adams-shaun/gorge/state"
)

const traceSchemaVersion = 1

type traceRunV1 struct {
	RecordType         string   `json:"record_type"`
	SchemaVersion      int      `json:"schema_version"`
	BoardSchemaVersion int      `json:"board_schema_version"`
	PolicyA            string   `json:"policy_a"`
	PolicyB            string   `json:"policy_b"`
	BaseSeed           uint64   `json:"base_seed"`
	GamesPerPair       int      `json:"games_per_pair"`
	Seats              int      `json:"seats"`
	Format             string   `json:"format"`
	MaxTurns           int      `json:"max_turns"`
	MaxIntents         int      `json:"max_intents"`
	Split              string   `json:"split,omitempty"`
	Suite              string   `json:"suite,omitempty"`
	Pairs              []string `json:"pairs"`
}

func newTraceRunV1(baseSeed uint64, games int, aName, bName string, pairs []pairDef, maxTurns, maxIntents int, commander bool) traceRunV1 {
	format := "constructed"
	if commander {
		format = "commander"
	}
	r := traceRunV1{
		RecordType: "run-v1", SchemaVersion: traceSchemaVersion, BoardSchemaVersion: traceSchemaVersion,
		PolicyA: aName, PolicyB: bName, BaseSeed: baseSeed, GamesPerPair: games, Seats: 2,
		Format: format, MaxTurns: maxTurns, MaxIntents: maxIntents,
	}
	for _, pair := range pairs {
		r.Pairs = append(r.Pairs, pair.String())
	}
	if samePairManifest(pairs, initialMono5Pairs()) {
		r.Suite = "mono5"
		switch {
		case baseSeed == 0 && games == 100:
			r.Split = "development"
		case baseSeed == 1_000_000 && games == 400:
			r.Split = "heldout"
		}
	}
	return r
}

func initialMono5Pairs() []pairDef {
	return fullPairs([]string{"mono-white-equipment", "mono-blue-tempo", "mono-black-aggro", "mono-red-prowess", "mono-green-stompy"})
}

func samePairManifest(a, b []pairDef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type traceDecisionMeta struct {
	PairIndex int
	Pair      string
	GameIndex int
	Seed      uint64
	Policy    string
}

type gameTrace struct {
	Decisions []traceDecisionV1
	Terminal  traceGameV1
}

func newGameTrace() *gameTrace { return &gameTrace{} }

func (g *gameTrace) finish(o gameOutcome, meta traceDecisionMeta) {
	r := traceGameV1{
		RecordType: "game-v1", SchemaVersion: traceSchemaVersion,
		PairIndex: meta.PairIndex, Pair: meta.Pair, GameIndex: meta.GameIndex, Seed: meta.Seed,
		Draw: o.winner == "" && !o.isStalled(), Stall: o.stallOn, Turns: o.turns,
		Intents: o.intents, Livelock: o.livelock,
	}
	if o.winner != "" {
		winner := state.PlayerID(o.winnerSeat)
		r.WinnerSeat = &winner
	}
	if o.starterSet {
		starter := o.starter
		r.StartingSeat = &starter
	}
	g.Terminal = r
}

type traceDecisionV1 struct {
	RecordType    string                 `json:"record_type"`
	SchemaVersion int                    `json:"schema_version"`
	PairIndex     int                    `json:"pair_index"`
	Pair          string                 `json:"pair"`
	GameIndex     int                    `json:"game_index"`
	Seed          uint64                 `json:"seed"`
	Sequence      uint64                 `json:"decision_sequence"`
	Seat          state.PlayerID         `json:"seat"`
	Policy        string                 `json:"policy"`
	Kind          decision.Kind          `json:"kind"`
	Min           int                    `json:"min"`
	Max           int                    `json:"max"`
	Options       []traceOptionV1        `json:"options"`
	Choices       []int                  `json:"choices"`
	TargetEffect  *decision.TargetEffect `json:"target_effect,omitempty"`
	Board         traceBoardV1           `json:"board"`
}

type traceOptionV1 struct {
	Index        int            `json:"index"`
	Kind         string         `json:"kind"`
	Object       state.ObjID    `json:"object,omitempty"`
	Player       state.PlayerID `json:"player"`
	Attacker     state.ObjID    `json:"attacker,omitempty"`
	Required     bool           `json:"required,omitempty"`
	Group        string         `json:"group,omitempty"`
	AltCostIndex int            `json:"alt_cost_index,omitempty"`
	CastMode     string         `json:"cast_mode,omitempty"`
	Amount       int            `json:"amount,omitempty"`
	AbilityIndex int            `json:"ability_index,omitempty"`
}

// traceBoardV1 aliases the shared board schema (internal/traceboard), the
// one type every decision-trace and label-corpus consumer decodes; the
// projection itself lives there too so the two writers cannot drift.
type traceBoardV1 = traceboard.Board

type traceGameV1 struct {
	RecordType    string          `json:"record_type"`
	SchemaVersion int             `json:"schema_version"`
	PairIndex     int             `json:"pair_index"`
	Pair          string          `json:"pair"`
	GameIndex     int             `json:"game_index"`
	Seed          uint64          `json:"seed"`
	WinnerSeat    *state.PlayerID `json:"winner_seat,omitempty"`
	Draw          bool            `json:"draw"`
	Stall         string          `json:"stall,omitempty"`
	Turns         int32           `json:"turns"`
	Intents       int             `json:"intents"`
	StartingSeat  *int            `json:"starting_seat,omitempty"`
	Livelock      string          `json:"livelock,omitempty"`
}

type traceTempFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
	Name() string
}

type traceWriterOps struct {
	createTemp func(string, string) (traceTempFile, error)
	publish    func(string, string) error
}

func defaultTraceWriterOps() traceWriterOps {
	return traceWriterOps{
		createTemp: func(parent, pattern string) (traceTempFile, error) {
			return os.CreateTemp(parent, pattern)
		},
		publish: publishTraceNoReplace,
	}
}

// publishTraceNoReplace makes the completed temporary file visible at dst
// atomically without the overwrite race of a check followed by os.Rename.
// The temporary file is a sibling, so the hard link stays on one filesystem.
func publishTraceNoReplace(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		// Publication is all-or-nothing to the caller: if retiring the temporary
		// name fails, remove the new name and let the deferred cleanup retry src.
		if rollbackErr := os.Remove(dst); rollbackErr != nil {
			return fmt.Errorf("removing published trace after temporary cleanup failed: %v (cleanup error: %w)", rollbackErr, err)
		}
		return err
	}
	return nil
}

func writeDecisionTrace(path string, run traceRunV1, games []*gameTrace) error {
	return writeDecisionTraceWithOps(path, run, games, defaultTraceWriterOps())
}

func writeDecisionTraceWithOps(path string, run traceRunV1, games []*gameTrace, ops traceWriterOps) (err error) {
	if path == "" {
		return nil
	}
	parent := filepath.Dir(path)
	info, statErr := os.Stat(parent)
	if statErr != nil {
		return fmt.Errorf("decision trace parent: %w", statErr)
	}
	if !info.IsDir() {
		return fmt.Errorf("decision trace parent %q is not a directory", parent)
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return fmt.Errorf("decision trace destination %q already exists", path)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("checking decision trace destination: %w", statErr)
	}
	if err := validateTraceRecord(run); err != nil {
		return err
	}
	tmp, err := ops.createTemp(parent, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating decision trace temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	enc := json.NewEncoder(tmp)
	encode := func(record any) error {
		if err := validateTraceRecord(record); err != nil {
			return err
		}
		if err := enc.Encode(record); err != nil {
			return fmt.Errorf("writing decision trace: %w", err)
		}
		return nil
	}
	if err = encode(run); err != nil {
		return err
	}
	for _, game := range games {
		if game == nil {
			return fmt.Errorf("nil game trace")
		}
		for _, record := range game.Decisions {
			if err = encode(record); err != nil {
				return err
			}
		}
		if err = encode(game.Terminal); err != nil {
			return err
		}
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("syncing decision trace: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing decision trace: %w", err)
	}
	tmp = nil
	if _, statErr := os.Lstat(path); statErr == nil {
		return fmt.Errorf("decision trace destination %q appeared during run", path)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("rechecking decision trace destination: %w", statErr)
	}
	if err = ops.publish(tmpName, path); err != nil {
		return fmt.Errorf("publishing decision trace: %w", err)
	}
	return nil
}

func (g *gameTrace) record(d *decision.Decision, in decision.Intent, b *botpolicy.Board, meta traceDecisionMeta) error {
	r := traceDecisionV1{
		RecordType: "decision-v1", SchemaVersion: traceSchemaVersion,
		PairIndex: meta.PairIndex, Pair: meta.Pair, GameIndex: meta.GameIndex, Seed: meta.Seed,
		Sequence: d.Seq, Seat: d.Player, Policy: meta.Policy, Kind: d.Kind, Min: d.Min, Max: d.Max,
		Choices: append([]int(nil), in.Choices...), Board: projectTraceBoard(b),
	}
	if d.TargetEffect != nil {
		r.TargetEffect = &decision.TargetEffect{API: d.TargetEffect.API}
		if d.TargetEffect.Damage != nil {
			r.TargetEffect.Damage = &decision.DamageEffect{}
			if d.TargetEffect.Damage.Amount != nil {
				n := *d.TargetEffect.Damage.Amount
				r.TargetEffect.Damage.Amount = &n
			}
		}
		if d.TargetEffect.Removal != nil {
			r.TargetEffect.Removal = &decision.RemovalEffect{
				Kind: d.TargetEffect.Removal.Kind, Destination: d.TargetEffect.Removal.Destination,
			}
		}
	}
	for _, o := range d.Options {
		r.Options = append(r.Options, traceOptionV1{
			Index: o.Index, Kind: o.Kind, Object: o.Obj, Player: o.Player, Attacker: o.Attacker,
			Required: o.Required, Group: o.Group, AltCostIndex: o.AltCostIndex, CastMode: o.Mode,
			Amount: o.Amount, AbilityIndex: o.Ability,
		})
	}
	if err := validateTraceRecord(r); err != nil {
		return err
	}
	g.Decisions = append(g.Decisions, r)
	return nil
}

func projectTraceBoard(b *botpolicy.Board) traceBoardV1 { return traceboard.Project(b) }

func validateTraceRecord(v any) error {
	switch r := v.(type) {
	case traceDecisionV1:
		if r.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("decision trace schema version %d is unsupported", r.SchemaVersion)
		}
		if r.RecordType != "decision-v1" {
			return fmt.Errorf("invalid decision trace record type %q", r.RecordType)
		}
		if r.Min < 0 || r.Max < r.Min {
			return fmt.Errorf("invalid decision choice bounds %d..%d", r.Min, r.Max)
		}
		for i, o := range r.Options {
			if o.Index != i {
				return fmt.Errorf("decision option %d has index %d", i, o.Index)
			}
		}
		for _, choice := range r.Choices {
			if choice < 0 || choice >= len(r.Options) {
				return fmt.Errorf("decision choice %d out of range", choice)
			}
		}
		if r.Board.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("board trace schema version %d is unsupported", r.Board.SchemaVersion)
		}
		return nil
	case traceGameV1:
		if r.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("game trace schema version %d is unsupported", r.SchemaVersion)
		}
		if r.RecordType != "game-v1" {
			return fmt.Errorf("invalid game trace record type %q", r.RecordType)
		}
		return nil
	case traceRunV1:
		if r.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("run trace schema version %d is unsupported", r.SchemaVersion)
		}
		if r.BoardSchemaVersion != traceSchemaVersion {
			return fmt.Errorf("board trace schema version %d is unsupported", r.BoardSchemaVersion)
		}
		if r.RecordType != "run-v1" {
			return fmt.Errorf("invalid run trace record type %q", r.RecordType)
		}
		return nil
	default:
		return fmt.Errorf("unsupported trace record %T", v)
	}
}
