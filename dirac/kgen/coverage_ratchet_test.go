package kgen

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const covHeader = "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n"

func covFS(rows, legacy, floor string, refused bool) fstest.MapFS {
	fsys := fstest.MapFS{
		"dirac/support/evidence/p.txt":           {Data: []byte("A\nB\n")},
		"dirac/support/evidence-keys/p.tsv":      {Data: []byte(covHeader + rows)},
		"dirac/support/evidence-keys/LEGACY.txt": {Data: []byte(legacy)},
		"dirac/support/coverage-floor.txt":       {Data: []byte(floor)},
	}
	if refused {
		fsys["dirac/support/refused/z.json"] = &fstest.MapFile{Data: []byte(`{"card":"Z","class":"c","why":"w","repro":{"pool":"p","surface":"full","seed":1,"pair":0,"policy":"random","mulligans":0}}`)}
	}
	return fsys
}

const zeroFloor = "keys_used=0\nkeys_total=0\nbranches_hit=0\nbranches_total=0\nfull=0\nstale_max=9\n"

const covFloor = "keys_used=3\nkeys_total=4\nbranches_hit=1\nbranches_total=2\nfull=1\nstale_max=1\n"

func TestCoverageRatchetCatches(t *testing.T) {
	lirNow := map[string]string{"A": "aaaaaaaaaaaaaaaa", "B": "bbbbbbbbbbbbbbbb", "Z": "zzzzzzzzzzzzzzzz"}
	rowA := "A\taaaaaaaaaaaaaaaa\t3\t2/2\t\t1/1\t\n"
	rowB := "B\tbbbbbbbbbbbbbbbb\t3\t1/2\t\t0/1\tix:x\n"
	for _, c := range []struct {
		name string
		fsys fstest.MapFS
		want string // substring of one fall, "" for none
	}{
		{"clean", covFS(rowA+rowB, "", covFloor, false), ""},
		{"lost evidence", covFS(rowA+rowB+"Y\tyyyyyyyyyyyyyyyy\t1\t1/1\t\t0/0\t\n", "", covFloor, false), "lost evidence: Y"},
		{"refused row is not lost", covFS(rowA+rowB+"Z\tzzzzzzzzzzzzzzzz\t1\t0/0\t\t0/0\t\n", "", covFloor, true), ""},
		{"no record", covFS(rowA, "", zeroFloor, false), "no coverage record: B"},
		{"legacy covers a missing row", covFS(rowA, "B\n", zeroFloor, false), ""},
		{"stale legacy", covFS(rowA+rowB, "B\n", covFloor, false), "stale legacy entry: B"},
		{"keys fell", covFS(rowA+rowB, "", strings.Replace(covFloor, "keys_used=3", "keys_used=4", 1), false), "keys fell"},
		{"branches fell", covFS(rowA+rowB, "", strings.Replace(covFloor, "branches_hit=1", "branches_hit=2", 1), false), "branches fell"},
		{"full fell", covFS(rowA+rowB, "", strings.Replace(covFloor, "full=1", "full=2", 1), false), "full fell"},
		{"stale rose", covFS(rowA+rowB, "", strings.Replace(covFloor, "stale_max=1", "stale_max=0", 1), false), "stale rose"},
	} {
		lir := lirNow
		if c.name == "stale rose" {
			lir = map[string]string{"A": "aaaaaaaaaaaaaaaa", "B": "0000000000000000"}
		}
		falls, err := coverageFalls(c.fsys, lir)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.want == "" {
			if len(falls) != 0 {
				t.Errorf("%s: falls %q, want none", c.name, falls)
			}
			continue
		}
		if !slices.ContainsFunc(falls, func(f string) bool { return strings.Contains(f, c.want) }) {
			t.Errorf("%s: falls %q, want one with %q", c.name, falls, c.want)
		}
		if !slices.IsSorted(falls) {
			t.Errorf("%s: falls not sorted: %q", c.name, falls)
		}
	}
}

// TestEvidenceCoverageRatchet holds each evidenced card's recorded coverage: the sidecar rows
// supportFalls cannot see. DIRAC_UPDATE_SUPPORT=1 raises coverage-floor.txt when nothing fell.
func TestEvidenceCoverageRatchet(t *testing.T) {
	heavyParallel(t)
	cardsDir := ratchetCards(t)
	fsys := os.DirFS(testRoot)
	names, err := coverageRowNames(fsys)
	if err != nil {
		t.Fatal(err)
	}
	lirNow := map[string]string{}
	if len(names) > 0 {
		if lirNow, err = LIRKeysFor(cardsDir, names); err != nil {
			t.Fatal(err)
		}
	}
	falls, err := coverageFalls(fsys, lirNow)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DIRAC_UPDATE_SUPPORT") == "1" && len(falls) == 0 {
		now, err := coverageNow(fsys, lirNow)
		if err != nil {
			t.Fatal(err)
		}
		floor, err := readCoverageFloor(fsys)
		if err != nil {
			t.Fatal(err)
		}
		if now.keysTotal >= floor.keysTotal && now.branchesTotal >= floor.branchesTotal {
			floor.keysUsed, floor.keysTotal = now.keysUsed, now.keysTotal
			floor.branchesHit, floor.branchesTotal, floor.full = now.branchesHit, now.branchesTotal, now.full
			if err := os.WriteFile(path.Join(testRoot, coverageFloorPath), []byte(floor.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	for _, f := range falls {
		t.Error(f)
	}
}
