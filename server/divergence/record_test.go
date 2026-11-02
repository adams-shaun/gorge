package divergence

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// sampleDivergences builds three divergences whose option lists carry the
// fields the wire shape must survive: an optional amount pointer, optional
// player/battle/attacker ids, a bare-tap option without a cost, and both
// nil and empty choice lists.
func sampleDivergences() []Divergence {
	return []Divergence{
		{
			RecordType:    RecordTypeDivergence,
			SchemaVersion: DivergenceSchemaVersion,
			ID:            "20261010T120000Z-t7-m1/118",
			Table:         "t7",
			Match:         1,
			Seat:          0,
			Intent:        118,
			EventSeq:      2291,
			DecisionSeq:   604,
			Turn:          9,
			Kind:          decision.KAttackers,
			Policy:        "bot",
			ShadowSeed:    1234567890123,
			Options: []decision.Option{
				{Index: 0, Kind: "attack", Label: "Attack with Grizzly Bears", Obj: 20, Player: 0, Amount: 2, Required: true},
				{Index: 1, Kind: "attack", Label: "Attack with Hill Giant", Obj: 21, Player: 1, Attacker: 30, Battle: 40},
			},
			Human:    Answer{Choices: []int{0, 2}, Label: "Attack with Grizzly Bears, Hill Giant"},
			Shadow:   Answer{Choices: []int{2}, Label: "Attack with Hill Giant"},
			Material: MaterialAttackSet,
		},
		{
			RecordType:    RecordTypeDivergence,
			SchemaVersion: DivergenceSchemaVersion,
			ID:            "20261010T120000Z-t7-m1/3",
			Table:         "t7",
			Match:         1,
			Seat:          0,
			Intent:        3,
			EventSeq:      41,
			DecisionSeq:   12,
			Turn:          1,
			Kind:          decision.KPriority,
			Policy:        "bot",
			ShadowSeed:    7,
			Options: []decision.Option{
				{Index: 0, Kind: "activate", Label: "Mountain", Obj: 5, Player: 0},
				{Index: 1, Kind: "pass", Label: "Pass"},
			},
			// A nil choice list must survive the round trip as nil.
			Human:    Answer{Label: "pass"},
			Shadow:   Answer{Choices: []int{}, Label: "tap Mountain"},
			Material: MaterialCastVsPass,
		},
		{
			RecordType:    RecordTypeDivergence,
			SchemaVersion: DivergenceSchemaVersion,
			ID:            "20261010T120000Z-t7-m1/57",
			Table:         "t7",
			Match:         1,
			Seat:          1,
			Intent:        57,
			EventSeq:      900,
			DecisionSeq:   300,
			Turn:          4,
			Kind:          decision.KTarget,
			Policy:        "bot",
			ShadowSeed:    99,
			Options: []decision.Option{
				{Index: 0, Kind: "creature", Label: "Grizzly Bears", Obj: 40, Player: 1},
			},
			Human:    Answer{Choices: []int{0}, Label: "Grizzly Bears"},
			Shadow:   Answer{Choices: []int{0}, Label: "Grizzly Bears"},
			Material: MaterialTarget,
		},
	}
}

// TestRecordRoundTripBytes pins the record format's determinism contract:
// marshal → unmarshal → marshal is byte-identical for both record types,
// the writer emits one compact line per record sorted by intent index no
// matter the input order, and a read gives back records that re-marshal to
// the same bytes.
func TestRecordRoundTripBytes(t *testing.T) {
	ds := sampleDivergences()

	var first bytes.Buffer
	if err := WriteDivergences(&first, ds); err != nil {
		t.Fatalf("write: %v", err)
	}

	// File order is by intent: 3, 57, 118.
	var wantOrder []string
	for _, want := range []string{`"intent":3,`, `"intent":57,`, `"intent":118,`} {
		wantOrder = append(wantOrder, want)
	}
	lines := strings.Split(strings.TrimSuffix(first.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines, want 3", len(lines))
	}
	for i, want := range wantOrder {
		if !strings.Contains(lines[i], want) {
			t.Fatalf("line %d lacks %s; file not sorted by intent:\n%s", i, want, lines[i])
		}
	}

	got, err := ReadDivergences(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != len(ds) {
		t.Fatalf("read %d records, want %d", len(got), len(ds))
	}
	var second bytes.Buffer
	if err := WriteDivergences(&second, got); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("round trip not byte-identical:\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}

	// Spot-check the shape against the human-divergence design §5.6's example keys.
	if !strings.Contains(lines[2], `"record_type":"divergence-v1"`) ||
		!strings.Contains(lines[2], `"schema_version":1`) ||
		!strings.Contains(lines[2], `"shadow_seed":1234567890123`) ||
		!strings.Contains(lines[2], `"material":"attack-set"`) {
		t.Fatalf("divergence line missing its §5.6 keys:\n%s", lines[2])
	}

	// Same contract for triage records, including a raw evaluation JSON
	// blob (the hindsight arm payload arrives verbatim).
	ts := []Triage{
		{
			RecordType:    RecordTypeTriage,
			SchemaVersion: TriageSchemaVersion,
			ID:            "20261010T120000Z-t7-m1/118",
			Intent:        118,
			Seat:          0,
			Bucket:        BucketHumanBetter,
			Material:      MaterialAttackSet,
			Candidates:    []Answer{{Choices: []int{2}}, {Choices: []int{0, 2}}},
			Evaluations: map[string]json.RawMessage{
				"honest":     json.RawMessage(`{"options":[{"index":0,"rate":0.5}]}`),
				"omniscient": json.RawMessage(`{"options":[{"index":1,"rate":0.75}]}`),
			},
			Provenance: Provenance{Commit: "abc", ForgeRef: "95f04e8a"},
		},
		{
			RecordType:    RecordTypeTriage,
			SchemaVersion: TriageSchemaVersion,
			ID:            "20261010T120000Z-t7-m1/3",
			Intent:        3,
			Bucket:        BucketUntriaged,
			Status:        StatusShadowDiverged,
		},
	}
	var tFirst bytes.Buffer
	if err := WriteTriages(&tFirst, ts); err != nil {
		t.Fatalf("write triages: %v", err)
	}
	tGot, err := ReadTriages(bytes.NewReader(tFirst.Bytes()))
	if err != nil {
		t.Fatalf("read triages: %v", err)
	}
	var tSecond bytes.Buffer
	if err := WriteTriages(&tSecond, tGot); err != nil {
		t.Fatalf("rewrite triages: %v", err)
	}
	if !bytes.Equal(tFirst.Bytes(), tSecond.Bytes()) {
		t.Fatalf("triage round trip not byte-identical:\nfirst:\n%s\nsecond:\n%s", tFirst.String(), tSecond.String())
	}
	tLines := strings.Split(strings.TrimSuffix(tFirst.String(), "\n"), "\n")
	if len(tLines) != 2 || !strings.Contains(tLines[0], `"intent":3,`) {
		t.Fatalf("triage file not sorted by intent:\n%s", tFirst.String())
	}
	if !strings.Contains(tLines[1], `"bucket":"human_better"`) {
		t.Fatalf("triage line missing its bucket:\n%s", tLines[1])
	}

	// The writer must not disturb the caller's slice order.
	if ds[0].Intent != 118 {
		t.Fatalf("WriteDivergences reordered its input")
	}
}

// TestRecordSortTiesBreakDeterministically pins the tie-break order: same
// intent, different seats and ids keep one stable relative order.
func TestRecordSortTiesBreakDeterministically(t *testing.T) {
	mk := func(seat state.PlayerID, id string) Divergence {
		return Divergence{
			RecordType:    RecordTypeDivergence,
			SchemaVersion: DivergenceSchemaVersion,
			ID:            id,
			Intent:        5,
			Seat:          seat,
		}
	}
	ds := []Divergence{mk(1, "b"), mk(0, "z"), mk(0, "a")}
	var buf bytes.Buffer
	if err := WriteDivergences(&buf, ds); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDivergences(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []string{"a", "z", "b"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d = %q, want %q", i, got[i].ID, id)
		}
	}
}

// TestReadRejectsGarbage pins that a corrupt line is an error, never a
// silently dropped record.
func TestReadRejectsGarbage(t *testing.T) {
	if _, err := ReadDivergences(strings.NewReader("{not json}\n")); err == nil {
		t.Fatalf("garbage line accepted")
	}
	if _, err := ReadTriages(strings.NewReader("[1,2]\n")); err == nil {
		t.Fatalf("wrong-shape line accepted")
	}
}
