package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// syntheticVisits is a tiny entity-encoded visit corpus for the pack goldens:
// two games, three records, and kinds whose first-seen order (target,
// priority, attackers) differs from the canonical table's, so a pack that
// indexed kinds first-seen would not match the dzg-pack-2 golden. The encoder
// hash is a fixed stand-in (syntheticHash), so the layout goldens do not move
// when the entity encoder does.
func syntheticVisits() []policynet.VisitRecord {
	base := func(game string, seq uint64, kind decision.Kind) policynet.VisitRecord {
		return policynet.VisitRecord{
			RecordType: policynet.VisitRecordType, SchemaVersion: policynet.VisitSchemaVersion,
			EncoderHash: syntheticHash, World: "redeal", Sims: 8, GameID: game,
			Deck: "red|green", Seed: 42, Opponent: "bot", Kind: kind, Turn: int32(seq), Sequence: seq,
		}
	}
	opt := func(row uint16, bot bool) policynet.OnPolicyOption {
		return policynet.OnPolicyOption{SlotsR: []uint16{row}, SlotsV: []float32{1}, HashedR: []uint16{row + 7},
			HashedV: []float32{0.5}, Dense: []float32{1, 0, 0.25}, BotPick: bot, InSpace: true}
	}
	r0 := base("p0000000g0", 1, decision.KTarget)
	r0.State = policynet.OnPolicyState{Dense: []float32{1, 2}, Rows: []uint16{3, 9}, Vals: []float32{1, 1},
		Cards: []policynet.OnPolicyCard{{Group: 1, Raw: []float32{0.5, 0.25}, Rows: []uint16{11}, Vals: []float32{1}}}}
	r0.Options = []policynet.OnPolicyOption{opt(1, true), opt(2, false)}
	r0.Options[1].EntA = 1
	r0.Cands, r0.Visits, r0.Prior, r0.Q = [][]int{{0}, {1}}, []int{5, 3}, []float64{0.6, 0.4}, []float64{0.5, 0.25}
	r0.RootValue, r0.Choice, r0.Outcome, r0.OutcomeKnown = 0.4, 0, 1, true

	r1 := base("p0000000g0", 2, decision.KPriority)
	r1.Seat = 1
	r1.State = policynet.OnPolicyState{Dense: []float32{0, 1}, Rows: []uint16{4}, Vals: []float32{2},
		Cards: []policynet.OnPolicyCard{{Group: 0, Raw: []float32{1}}, {Group: 2, Raw: []float32{0, 1}, Rows: []uint16{12, 13}, Vals: []float32{1, 1}}}}
	r1.Options = []policynet.OnPolicyOption{opt(3, true), opt(4, false), opt(5, false)}
	r1.Cands, r1.Visits, r1.Prior, r1.Q = [][]int{{0}, {1}, {2}}, []int{1, 6, 1}, []float64{0.2, 0.5, 0.3}, []float64{0.1, 0.7, 0.2}
	r1.RootValue, r1.Choice, r1.Outcome, r1.OutcomeKnown = 0.6, 1, 0, true

	r2 := base("p0000000g1", 3, decision.KAttackers)
	r2.Subset = true
	r2.State = policynet.OnPolicyState{Dense: []float32{0.5}, Rows: []uint16{5}, Vals: []float32{1},
		Cards: []policynet.OnPolicyCard{{Group: 3, Raw: []float32{0.75}}}}
	r2.Options = []policynet.OnPolicyOption{opt(6, true), opt(8, false)}
	r2.Cands, r2.Visits, r2.Prior, r2.Q = [][]int{{}, {0}, {0, 1}}, []int{2, 2, 4}, []float64{0.3, 0.3, 0.4}, []float64{0.4, 0.5, 0.6}
	r2.RootValue, r2.Choice = 0.55, 2 // outcome unknown: a halted game
	return []policynet.VisitRecord{r0, r1, r2}
}

const syntheticHash = "0123456789abcdef"

// writeVisitCorpus writes recs as one gzip member, one JSON record per line,
// the way play -corpus does (corpusMember).
func writeVisitCorpus(t *testing.T, path string, recs []policynet.VisitRecord) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	enc := json.NewEncoder(zw)
	for i := range recs {
		if err := enc.Encode(&recs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readDir maps every file under dir (relative path) to its bytes.
func readDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
