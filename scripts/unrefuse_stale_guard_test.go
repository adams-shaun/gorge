package scripts

// TestUnrefuseStaleKeepsLockstepCards drives scripts/unrefuse-stale.sh over a fake tree
// holding two cards stale at once: a locked one, named by a Test<Something>ReprosLockstep
// fixture (the shape mbc_fire_dragon_lockstep_test.go has), and a free one, named only by
// an ordinary test. It asserts the guard keeps the locked card's refusal entry, still
// unrefuses the free one, and names the card and its test on one line. Without the guard
// the script unrefuses both, which cannot be green: each pins its entry.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	guardLockedCard = "Guard Locked Card"
	guardFreeCard   = "Guard Free Card"
	guardLockedSlug = "guard-locked-card"
	guardFreeSlug   = "guard-free-card"
	guardSha        = "abc1234def"
)

func TestUnrefuseStaleKeepsLockstepCards(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"dirac/support", "dirac/oracle/testdata/repro", "stale"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "guard_unrefuse", "fixture_lockstep_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dirac", "oracle", "fixture_lockstep_test.go"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	repro := []byte(`{"format":"koracle-repro-2","seed":9}`)
	for _, slug := range []string{guardLockedSlug, guardFreeSlug} {
		if err := os.WriteFile(filepath.Join(root, "stale", slug+".repro.json"), repro, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stale := guardLockedCard + "\t" + guardLockedSlug + "\t12\tstd-trk\n" +
		guardFreeCard + "\t" + guardFreeSlug + "\t12\tstd-trk\n"
	if err := os.WriteFile(filepath.Join(root, "stale", "stale.tsv"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(root, "dirac", "support", "unrefused.tsv")
	header := "# Cards that had a dirac/support/refused entry and no longer do, with the proof the defect is fixed.\n"
	if err := os.WriteFile(reg, []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}

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
	cmd := exec.Command("bash", script, filepath.Join(root, "stale"), guardSha)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GORGE_ROOT="+root, "KGEN="+stub, "STUB_LOG="+stubLog)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", unrefuseStaleScript, err, out)
	}

	// The guard keeps the locked card: kgen -unrefuse ran only for the free one.
	got, err := os.ReadFile(stubLog)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stub log: %v", err)
	}
	if want := "-unrefuse " + guardFreeCard + "\n"; string(got) != want {
		t.Fatalf("kgen saw %q, want only %q", got, want)
	}
	// The locked card's recorded repro is left where it is; the free card's moved as proof.
	if _, err := os.Stat(filepath.Join(root, "stale", guardLockedSlug+".repro.json")); err != nil {
		t.Fatalf("the locked card's repro moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dirac", "oracle", "testdata", "repro", guardFreeSlug+"_auto_unrefused.repro.json")); err != nil {
		t.Fatalf("the free card's proof repro did not land: %v", err)
	}
	// Only the free card gets the register row.
	body, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), guardLockedCard) {
		t.Fatalf("the locked card gained an unrefused.tsv row: %q", body)
	}
	if !strings.Contains(string(body), guardFreeCard) {
		t.Fatalf("the free card gained no unrefused.tsv row: %q", body)
	}
	// The guard says why it kept the entry, naming the card and its test.
	wantLine := "unrefused=no card=" + guardLockedCard + " test=TestGuardLockedCardReprosLockstep"
	if !strings.Contains(string(out), wantLine) {
		t.Fatalf("output %q lacks %q", out, wantLine)
	}
}
