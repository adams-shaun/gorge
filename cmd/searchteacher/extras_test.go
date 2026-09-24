package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// extrasArgs is a cheap oracle run teaching attackers and cast (the pn12
// recipe's kinds) over the first eight turns.
func extrasArgs(t *testing.T, labels string, workers int, extras bool) []string {
	args := labelTestArgs(t, labels, workers)
	for i := range args {
		if args[i] == "-kinds" {
			args[i+1] = "attackers,cast"
		}
		if args[i] == "-max-turn" {
			args[i+1] = "8"
		}
	}
	args = append(args, "-oracle")
	if extras {
		args = append(args, "-label-extras")
	}
	return args
}

func gunzipLines(t *testing.T, path string) [][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
}

// -label-extras writes schema 3 records that are the schema 2 records plus
// the extras object and nothing else (the extras never move the game), the
// .gz corpus is byte-identical across worker counts, and the extras carry
// what the pn12 grid re-encodes from.
func TestLabelExtrasCorpus(t *testing.T) {
	testutil.CorpusRegistry(t) // skips on a clean clone with no .cards/
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.jsonl")
	x1 := filepath.Join(dir, "x1.jsonl.gz")
	x3 := filepath.Join(dir, "x3.jsonl.gz")
	for _, r := range []struct {
		path    string
		workers int
		extras  bool
	}{{plain, 2, false}, {x1, 1, true}, {x3, 3, true}} {
		if err := run(extrasArgs(t, r.path, r.workers, r.extras), io.Discard, io.Discard); err != nil {
			t.Fatalf("%s: %v", r.path, err)
		}
	}
	a, _ := os.ReadFile(x1)
	b, _ := os.ReadFile(x3)
	if !bytes.Equal(a, b) {
		t.Fatal("the -label-extras corpus differs between -workers 1 and 3")
	}
	base := readLabelRecords(t, plain)
	lines := gunzipLines(t, x1)
	if len(lines) != len(base) || len(base) == 0 {
		t.Fatalf("%d extras records vs %d plain records", len(lines), len(base))
	}
	var follow, chosen, priority int
	for i, line := range lines {
		var r LabelRecord
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		if r.SchemaVersion != labelSchemaExtras || r.Extras == nil {
			t.Fatalf("record %d: schema %d extras %v", i, r.SchemaVersion, r.Extras != nil)
		}
		if r.Extras.DiagOppHand == nil || r.Extras.DiagOwnLibraryTop == nil || len(r.Extras.DiagOppLibraryTop) != libraryPeek {
			t.Fatalf("record %d: diagnostic extras missing: %+v", i, r.Extras)
		}
		for _, ft := range r.Extras.FollowTargets {
			follow++
			if ft.Option < 0 || ft.Option >= len(r.Options) || len(ft.Targets) == 0 {
				t.Fatalf("record %d: follow target %+v out of range", i, ft)
			}
			for _, c := range ft.Choices {
				if c < 0 || c >= len(ft.Targets) {
					t.Fatalf("record %d: follow choice %d of %d", i, c, len(ft.Targets))
				}
			}
		}
		if r.Kind == "priority" {
			priority++
		}
		if r.Extras.ChosenTarget != nil {
			chosen++
		}
		// Strip the extras: what is left is the schema 2 record, byte for byte.
		r.Extras, r.SchemaVersion = nil, labelSchemaVersion
		got, _ := json.Marshal(r)
		want, _ := json.Marshal(base[i])
		if !bytes.Equal(got, want) {
			t.Fatalf("record %d: the extras run's record differs from the plain run's", i)
		}
	}
	if priority == 0 || follow == 0 {
		t.Fatalf("fixture no longer exercises the extras: %d priority records, %d follow targets, %d chosen", priority, follow, chosen)
	}
	// The loader reads the corpus under every feature set and the joint
	// encoding.
	for _, lo := range []policynet.LoadOptions{{}, {Features: policynet.FeaturesMZOracle}, {Joint: true}} {
		exs, _, err := policynet.LoadWith(x1, lo)
		if err != nil || len(exs) != len(lines) {
			t.Fatalf("LoadWith %+v: %d examples, %v", lo, len(exs), err)
		}
	}
}
