package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/internal/traceboard"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// The corpus fixture: one pair, two games, cheap teacher settings (K=2, 16
// attempts, 2 candidates, 3-turn horizon, teaching only the first six turns)
// so the whole suite stays inside the package's test budget while still
// exercising the real sampler and real rollouts. Seeds are development
// seeds; the held-out refusal is covered by TestRefusesHeldOutSeedRange.
const (
	labelTestA       = "mono-black-aggro"
	labelTestB       = "mono-green-stompy"
	labelTestSeed    = uint64(31_000_000)
	labelTestGameIdx = 0 // the game whose opponent's opening hand the redaction test replays
)

func labelTestArgs(t *testing.T, labels string, workers int) []string {
	t.Helper()
	cmd := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel")
	cmd.Env = cards.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolving repo root for the corpus dir: %v", err)
	}
	corpus := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("no .cards/ corpus at %s", corpus)
	}
	return []string{
		"-games", "2",
		"-pairs", labelTestA + ":" + labelTestB,
		"-kinds", "attackers",
		"-worlds", "2",
		"-attempts", "16",
		"-candidates", "2",
		"-horizon", "3",
		"-max-turn", "6",
		"-seed", strconv.FormatUint(labelTestSeed, 10),
		"-workers", strconv.Itoa(workers),
		"-labels", labels,
		"-cards", corpus,
	}
}

func readLabelRecords(t *testing.T, path string) []LabelRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []LabelRecord
	for i, line := range bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n")) {
		var r LabelRecord
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if !bytes.Contains(line, []byte(`"record_type":"label-v1"`)) || r.SchemaVersion != labelSchemaVersion {
			t.Fatalf("record %d: record_type/schema_version %q/%d", i, r.RecordType, r.SchemaVersion)
		}
		out = append(out, r)
	}
	return out
}

func TestLabelsCorpus(t *testing.T) {
	reg := testutil.CorpusRegistry(t) // skips on a clean clone with no .cards/
	dir := t.TempDir()
	pathW1 := filepath.Join(dir, "labels-w1.jsonl")
	if err := run(labelTestArgs(t, pathW1, 1), io.Discard, io.Discard); err != nil {
		t.Fatalf("workers 1 run: %v", err)
	}
	records := readLabelRecords(t, pathW1)
	if len(records) == 0 {
		t.Fatalf("no label records at seed %d; the fixture stopped exercising the corpus", labelTestSeed)
	}

	t.Run("candidates", func(t *testing.T) {
		for i, r := range records {
			if len(r.Candidates) < 2 {
				t.Fatalf("record %d (seq %d): %d candidates, want >=2", i, r.Sequence, len(r.Candidates))
			}
			if !r.Candidates[0].Bot || r.Candidates[0].Worlds != r.Worlds || r.Worlds < 1 {
				t.Fatalf("record %d: candidate 0 bot=%v worlds=%d (K=%d)", i, r.Candidates[0].Bot, r.Candidates[0].Worlds, r.Worlds)
			}
			for _, c := range r.Candidates {
				if c.Value < 0 || c.Value > 1 {
					t.Fatalf("record %d: candidate value %g outside [0,1]", i, c.Value)
				}
			}
			if r.TeacherChoice < 0 || r.TeacherChoice >= len(r.Candidates) || r.BotIndex != 0 {
				t.Fatalf("record %d: teacher_choice %d / bot_index %d over %d candidates", i, r.TeacherChoice, r.BotIndex, len(r.Candidates))
			}
			// Margin is the best candidate mean minus the bot's mean over ALL
			// candidates, independent of which one the teacher chose. A
			// nonzero margin with the bot kept (TeacherChoice == 0) is legal at
			// -margin > 0, so assert the real contract instead: margin is that
			// difference, and the teacher only ever chooses the best mean.
			best := r.Candidates[0].Value
			for j := 1; j < len(r.Candidates); j++ {
				if r.Candidates[j].Value > best {
					best = r.Candidates[j].Value
				}
			}
			if want := best - r.Candidates[0].Value; r.Margin != want {
				t.Fatalf("record %d: margin %g, want best minus bot %g", i, r.Margin, want)
			}
			if r.TeacherChoice != 0 && r.Candidates[r.TeacherChoice].Value != best {
				t.Fatalf("record %d: teacher chose candidate %d (value %g) but the best mean is %g",
					i, r.TeacherChoice, r.Candidates[r.TeacherChoice].Value, best)
			}
		}
	})

	t.Run("board is the botbench trace board", func(t *testing.T) {
		for i, r := range records {
			// decode the record's board field with the trace board type —
			// the very type botbench's traceDecisionV1.Board carries (the
			// trace aliases internal/traceboard.Board) — and require it to
			// agree with the record's own copy.
			var probe struct {
				Board traceboard.Board `json:"board"`
			}
			if err := json.Unmarshal(mustMarshal(t, r), &probe); err != nil {
				t.Fatalf("record %d: %v", i, err)
			}
			if probe.Board.SchemaVersion != traceboard.SchemaVersion {
				t.Fatalf("record %d: board schema version %d, want %d", i, probe.Board.SchemaVersion, traceboard.SchemaVersion)
			}
			if !bytes.Equal(mustMarshal(t, probe.Board), mustMarshal(t, r.Board)) {
				t.Fatalf("record %d: board does not round-trip through the trace board type", i)
			}
		}
	})

	t.Run("round trip", func(t *testing.T) {
		data, err := os.ReadFile(pathW1)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n")) {
			var r LabelRecord
			if err := json.Unmarshal(line, &r); err != nil {
				t.Fatalf("record %d: %v", i, err)
			}
			if again := mustMarshal(t, r); !bytes.Equal(again, line) {
				t.Fatalf("record %d does not round-trip:\n got %s\nwant %s", i, again, line)
			}
		}
	})

	t.Run("deterministic across workers", func(t *testing.T) {
		pathW4 := filepath.Join(dir, "labels-w4.jsonl")
		if err := run(labelTestArgs(t, pathW4, 4), io.Discard, io.Discard); err != nil {
			t.Fatalf("workers 4 run: %v", err)
		}
		a, err := os.ReadFile(pathW1)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(pathW4)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("label corpus differs between -workers 1 and -workers 4")
		}
	})

	t.Run("view is redacted", func(t *testing.T) {
		// Replay game 0's opening deal directly: the opponent (seat 1) holds
		// known named cards before any mulligan, and none of the names still
		// hidden from the deciding seat may appear anywhere in a record's view.
		da, err := testutil.LoadRepoDeck(reg, labelTestA)
		if err != nil {
			t.Fatal(err)
		}
		db, err := testutil.LoadRepoDeck(reg, labelTestB)
		if err != nil {
			t.Fatal(err)
		}
		e := rules.New(rules.Config{Seed: labelTestSeed + labelTestGameIdx, Names: []string{labelTestA, labelTestB},
			Decks: [][]*cards.Card{da, db}, Tokens: reg.Tokens})
		e.Advance()
		if len(e.G.Players) != 2 {
			t.Fatalf("expected 2 seats, got %d", len(e.G.Players))
		}
		actorDeckNames := map[string]bool{}
		for _, c := range da {
			actorDeckNames[c.Faces[0].Name] = true
		}
		known := []string{}
		for _, id := range e.G.Zone(state.ZHand, 1) {
			name := e.G.Obj(id).Card.Faces[0].Name
			known = append(known, name)
		}
		if len(known) < 5 {
			t.Fatalf("opponent opening hand has %d cards; the fixture holds no known hidden cards", len(known))
		}
		hidden := map[string]bool{} // opening-hand names absent from the deciding seat's deck AND from the record's visible zones
		for _, name := range known {
			if !actorDeckNames[name] {
				hidden[name] = true
			}
		}
		anyHiddenHand := false
		for _, r := range records {
			if r.GameIndex != labelTestGameIdx || r.View == nil {
				continue
			}
			var v view.View
			if err := json.Unmarshal(r.View, &v); err != nil {
				t.Fatalf("record seq %d: view does not decode as view.View: %v", r.Sequence, err)
			}
			if v.Decision != nil {
				t.Fatalf("record seq %d: view carries a decision (continuation state)", r.Sequence)
			}
			opponent := 1 - int(r.Seat)
			visible := map[string]bool{}
			for _, p := range v.Players {
				if int(p.ID) == opponent {
					if p.Hand != nil {
						t.Fatalf("record seq %d: opponent hand is %v, want null (CR 400.2 redaction)", r.Sequence, p.Hand)
					}
					if p.HandSize > 0 {
						anyHiddenHand = true
					}
				}
				for _, c := range p.Battlefield {
					visible[c.Name] = true
				}
				for _, c := range p.Graveyard {
					visible[c.Name] = true
				}
				for _, c := range p.Exile {
					visible[c.Name] = true
				}
				for _, c := range p.Command {
					visible[c.Name] = true
				}
				if p.LibraryTop != nil {
					visible[p.LibraryTop.Name] = true
				}
			}
			for _, s := range v.Stack {
				visible[s.Name] = true
				if s.Card != nil {
					visible[s.Card.Name] = true
				}
			}
			for name := range hidden {
				if visible[name] {
					continue // legitimately on show in a public zone
				}
				if strings.Contains(string(r.View), name) {
					t.Fatalf("record seq %d: opponent hand card name %q leaked into the seat's view", r.Sequence, name)
				}
			}
		}
		if !anyHiddenHand {
			t.Fatalf("no record caught a non-empty hidden opponent hand; the redaction assertion would pass vacuously")
		}
	})
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return b
}

func TestLabelsRefusesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels.jsonl")
	if err := os.WriteFile(path, []byte("occupied\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(labelTestArgs(t, path, 1), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing destination: want refusal, got %v", err)
	}
}
