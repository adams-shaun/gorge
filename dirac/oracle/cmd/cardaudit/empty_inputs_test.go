package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// TestEmptyInputsYieldNoRecords: a sidecar, hunt or ledger file with no header
// line is zero records, not an error that aborts the audit.
func TestEmptyInputsYieldNoRecords(t *testing.T) {
	fsys := fstest.MapFS{
		"dirac/support/evidence-keys/empty.tsv":   {Data: nil},
		"dirac/support/evidence-keys/comment.tsv": {Data: []byte("# nothing yet\n")},
	}
	sc, err := readSidecars(fsys)
	if err != nil {
		t.Fatalf("readSidecars on header-less files: %v", err)
	}
	if len(sc) != 0 {
		t.Fatalf("readSidecars = %v, want no rows", sc)
	}

	dir := t.TempDir()
	for name, body := range map[string]string{"hunt.tsv": "", "led.csv": "", "led2.csv": "# c\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readHunt(filepath.Join(dir, "hunt.tsv")); err != nil {
		t.Fatalf("readHunt on an empty file: %v", err)
	}
	for _, n := range []string{"led.csv", "led2.csv"} {
		if _, err := readXLedger(filepath.Join(dir, n)); err != nil {
			t.Fatalf("readXLedger(%s) on a header-less file: %v", n, err)
		}
	}
}

// TestUnknownXBucketIsKept: a bucket value outside the three known ones is
// recorded as itself, ranks below AGREEING, and shows in the summary's
// "other" column instead of vanishing.
func TestUnknownXBucketIsKept(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "led.csv")
	body := "set,card,bucket,diff_primary\n" +
		"s1,Odd Card,weird-thing,0\n" +
		"s1,Mixed Card,weird-thing,0\n" +
		"s2,Mixed Card,agreeing,0\n"
	if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := readXLedger(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.bucket["Odd Card"]; got != "WEIRD-THING" {
		t.Fatalf("Odd Card bucket = %q, want the unknown value kept as WEIRD-THING", got)
	}
	if got := idx.bucket["Mixed Card"]; got != "AGREEING" {
		t.Fatalf("Mixed Card bucket = %q, want AGREEING (ranks above an unknown value)", got)
	}

	var sb strings.Builder
	rows := []row{{Card: "Odd Card", Keys: "-", Branches: "-", Missing: "-", XBucket: idx.bucket["Odd Card"]}}
	if err := writeSummary(&sb, rows); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "| other |") || !strings.Contains(sb.String(), "| 0 | 0 | 0 | 1 |\n") {
		t.Fatalf("summary does not count the unknown bucket under other:\n%s", sb.String())
	}
}
