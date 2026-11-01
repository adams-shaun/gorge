package policynet

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

var updateContracts = flag.Bool("update-contracts", false, "rewrite bots/policynet/testdata/contracts goldens")

// TestContractVisitRecord pins contract azvisits (a companion module's contract registry):
// one visit record's JSON line, every field set. A new field is additive
// (rewrite with -update-contracts, rehash the registry, CHANGELOG line); a
// renamed, retyped or removed one is VisitSchemaVersion 2.
func TestContractVisitRecord(t *testing.T) {
	rec := VisitRecord{
		RecordType: VisitRecordType, SchemaVersion: VisitSchemaVersion, EncoderHash: "0123456789abcdef",
		World: "redeal", Sims: 64, GameID: "p0000001g0", Deck: "red|green", Seed: 42, Seat: 1, Opponent: "bot",
		Kind: decision.KAttackers, Turn: 5, Sequence: 77, Subset: true,
		State: OnPolicyState{Dense: []float32{1, 0.5}, Rows: []uint16{3}, Vals: []float32{1},
			Cards: []OnPolicyCard{{Group: 2, Raw: []float32{0.25}, Rows: []uint16{9}, Vals: []float32{1}}}},
		DiagRows: []uint16{12}, DiagVals: []float32{1},
		Options: []OnPolicyOption{{SlotsR: []uint16{1}, SlotsV: []float32{1}, HashedR: []uint16{8}, HashedV: []float32{0.5},
			Dense: []float32{1, 0}, BotPick: true, InSpace: true, EntA: 1, EntB: 2}},
		Cands: [][]int{{}, {0}}, Visits: []int{10, 54}, Prior: []float64{0.5, 0.5}, Q: []float64{0.25, 0.75},
		RootValue: 0.7, Choice: 1, Outcome: 1, OutcomeKnown: true, Weight: 0.25,
	}
	got, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "contracts", "azvisits.jsonl")
	if *updateContracts {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the azvisits record changed (docs/contracts.md):\n got: %s\nwant: %s", got, want)
	}
	var back VisitRecord
	if err := json.Unmarshal(want, &back); err != nil || back.RecordType != VisitRecordType || back.Choice != 1 {
		t.Fatalf("golden does not read back: %v %+v", err, back)
	}
}
