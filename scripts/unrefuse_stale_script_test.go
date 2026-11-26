package scripts

// TestUnrefuseStale drives scripts/unrefuse-stale.sh on a fake tree: one stale line, a
// stub kgen (the KGEN override), a fixed short sha. It asserts the recorded repro moved
// into dirac/oracle/testdata/repro as the proof, kgen ran -unrefuse on the card, the
// proof row landed in dirac/support/unrefused.tsv, and an empty (or absent) stale.tsv
// changes nothing.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	unrefuseStaleScript = "unrefuse-stale.sh"
	unrefuseStaleCard   = "Frostboil Snare"
	unrefuseStaleSlug   = "frostboil-snare"
	unrefuseStaleSha    = "abc1234def"
)

// unrefuseStaleRun runs the script on root with a stub kgen and a fixed sha, and returns
// what the stub saw ("" when it never ran, which the callers assert on).
func unrefuseStaleRun(t *testing.T, root, staleDir string) string {
	t.Helper()
	tmp := t.TempDir()
	stubLog := filepath.Join(tmp, "stub.log")
	stub := filepath.Join(tmp, "kgen-stub.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$STUB_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(unrefuseStaleScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, staleDir, unrefuseStaleSha)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GORGE_ROOT="+root, "KGEN="+stub, "STUB_LOG="+stubLog)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", unrefuseStaleScript, err, out)
	}
	b, err := os.ReadFile(stubLog)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stub log: %v", err)
	}
	return string(b)
}

func TestUnrefuseStale(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"dirac/support", "dirac/oracle/testdata/repro", "stale"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repro := []byte(`{"actions":null,"cards":[[],[]],"decks":["deck-a","deck-b"],"format":"koracle-repro-2","seed":9}`)
	if err := os.WriteFile(filepath.Join(root, "stale", unrefuseStaleSlug+".repro.json"), repro, 0o644); err != nil {
		t.Fatal(err)
	}
	stale := unrefuseStaleCard + "\t" + unrefuseStaleSlug + "\t12\tstd-trk\n"
	if err := os.WriteFile(filepath.Join(root, "stale", "stale.tsv"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(root, "dirac", "support", "unrefused.tsv")
	// Precondition: the register holds its header and no rows, so the row the script
	// appends is the script's, not one already there.
	if err := os.WriteFile(reg, []byte("# Cards that had a dirac/support/refused entry and no longer do, with the proof the defect is fixed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := unrefuseStaleRun(t, root, filepath.Join(root, "stale")); got != "-unrefuse "+unrefuseStaleCard+"\n" {
		t.Fatalf("the stub kgen saw %q", got)
	}
	dst := filepath.Join(root, "dirac", "oracle", "testdata", "repro", unrefuseStaleSlug+"_auto_unrefused.repro.json")
	if b, err := os.ReadFile(dst); err != nil || string(b) != string(repro) {
		t.Fatalf("the proof repro at %s: %v (%q)", dst, err, b)
	}
	if _, err := os.Stat(filepath.Join(root, "stale", unrefuseStaleSlug+".repro.json")); !os.IsNotExist(err) {
		t.Fatalf("the recorded repro did not move: %v", err)
	}
	b, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	want := "dirac/oracle/testdata/repro/" + unrefuseStaleSlug + "_auto_unrefused.repro.json"
	if row := unrefuseStaleCard + "\t" + want + "\tauto-unrefused by regen-main: no divergence in 12 pairs of std-trk at " + unrefuseStaleSha + "\n"; string(b) != "# Cards that had a dirac/support/refused entry and no longer do, with the proof the defect is fixed.\n"+row {
		t.Fatalf("unrefused.tsv = %q, want it with %q", b, row)
	}
}

func TestUnrefuseStaleEmptyChangesNothing(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"dirac/support", "dirac/oracle/testdata/repro"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reg := filepath.Join(root, "dirac", "support", "unrefused.tsv")
	if err := os.WriteFile(reg, []byte("# header\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("no stale.tsv at all", func(t *testing.T) {
		unrefuseStaleRun(t, root, t.TempDir())
	})

	t.Run("an empty stale.tsv", func(t *testing.T) {
		staleDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(staleDir, "stale"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Precondition: a repro and a stale.tsv sit in the dir; the empty line file must
		// leave both where they are.
		repro := filepath.Join(staleDir, unrefuseStaleSlug+".repro.json")
		if err := os.WriteFile(repro, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staleDir, "stale.tsv"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		unrefuseStaleRun(t, root, staleDir)
		if _, err := os.Stat(repro); err != nil {
			t.Fatalf("an empty stale.tsv moved the repro: %v", err)
		}
	})

	b, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "# header\n" {
		t.Fatalf("nothing went stale, yet unrefused.tsv gained %q", b)
	}
}
