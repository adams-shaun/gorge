package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRecordWritesOneVerifiedRecordPerGame: -record DIR writes one
// gorge-match-1 record per game, and each one loads from the corpus and
// verifies as pure gorge through matchrecord.VerifyChance.
func TestRecordWritesOneVerifiedRecordPerGame(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	recDir := filepath.Join(t.TempDir(), "records")
	var out bytes.Buffer
	o := simOptions{Seats: 2, Seed: 3, Games: 2, Dir: dir, Format: "constructed", Record: recDir}
	if err := runSim(o, &out); err != nil {
		t.Fatalf("runSim: %v\n%s", err, out.String())
	}
	ents, err := os.ReadDir(recDir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), matchrecord.Ext) {
			paths = append(paths, filepath.Join(recDir, e.Name()))
		}
	}
	if len(paths) != 2 {
		t.Fatalf("-record wrote %d records for 2 games: %v", len(paths), ents)
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		r, cfg, err := matchrecord.Load(p, reg)
		if err != nil {
			t.Fatal(err)
		}
		if r.Producer.Tool != "mtgsim" || len(r.Producer.Policies) != 2 || r.Producer.ForgeRef != matchrecord.ForgeRef(dir) {
			t.Errorf("%s: producer %+v", p, r.Producer)
		}
		if err := matchrecord.VerifyChance(r, cfg); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}
