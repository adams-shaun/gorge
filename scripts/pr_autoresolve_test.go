package scripts

// TestAutoresolveMatcher: scripts/pr-autoresolve.sh resolves a conflict alone only when every conflicted path is a
// generated file in scripts/autoresolve-paths.tsv (--match prints the rest). A path the matcher wrongly accepts would
// be overwritten with main's side and pushed to a PR; one it wrongly rejects leaves the PR CONFLICTING.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoresolveMatcher(t *testing.T) {
	generated := []string{
		"dirac/docs/support.md",
		"dirac/kgen/testdata/acceptance.tsv",
		"dirac/kgen/testdata/build_acceptance.tsv",
		"dirac/kgen/testdata/remspan.tsv",
		"dirac/kop/testdata/ordinals.tsv",
		"dirac/testdata/layout.tsv",
		"internal/codeshape/testdata/func_ceilings.tsv",
		"dirac/oracle/testdata/repro_table_names.txt",
	}
	other := []string{
		"dirac/kgen/acceptance_test.go",
		"dirac/kgen/testdata/sub/acceptance.tsv",
		"dirac/manifest/testdata/tree/dirac/kop/testdata/ordinals.tsv",
		"dirac/docs/support.md.orig",
		"engine/rules/combat.go",
		"scripts/autoresolve-paths.tsv",
	}
	match := func(paths ...string) (string, bool) {
		out, err := exec.Command("bash", append([]string{"pr-autoresolve.sh", "--match"}, paths...)...).Output()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				t.Fatal(err)
			}
		}
		return strings.TrimSpace(string(out)), err == nil
	}
	if out, ok := match(generated...); !ok || out != "" {
		t.Errorf("generated paths rejected: %q", out)
	}
	for _, p := range other {
		if out, ok := match(append([]string{p}, generated...)...); ok || out != p {
			t.Errorf("%s: want it reported alone and a failure, got %q ok=%v", p, out, ok)
		}
	}
}

// TestAutoresolveUnionKeepsAddedRow: a PR that adds a list row main lacks (a new set list in an acceptance tsv) keeps
// it; main's rows win for shared keys; a row main deleted since the merge base stays deleted.
func TestAutoresolveUnionKeepsAddedRow(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	hdr := "list\taccepted\tcards\n"
	base := write("base", hdr+"ALL\t10\t20\nOLD\t1\t2\nBLB\t3\t4\n")
	main := write("main", hdr+"ALL\t12\t22\nBLB\t3\t4\n")
	pr := write("pr", hdr+"ALL\t11\t21\nBLB\t3\t4\nCM2\t5\t6\nOLD\t1\t2\n")
	out, err := exec.Command("bash", "pr-autoresolve.sh", "--union", main, pr, base).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := hdr + "ALL\t12\t22\nBLB\t3\t4\nCM2\t5\t6\n"
	if string(out) != want {
		t.Errorf("union:\n%q\nwant\n%q", out, want)
	}
}

// TestAutoresolveUnionKeepsKeyOrder: an added row lands in key order among main's rows, not at the end (11-25: the
// CM2 acceptance row appended after WOE failed parsePinTSV's byte-after check in every resolve job).
func TestAutoresolveUnionKeepsKeyOrder(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	hdr := "list\taccepted\tcards\n"
	base := write("base", hdr+"ALL\t10\t20\nBLB\t3\t4\n")
	main := write("main", hdr+"ALL\t12\t22\nBLB\t3\t4\nWOE\t1\t1\n")
	pr := write("pr", hdr+"ALL\t11\t21\nBLB\t3\t4\nCMM\t7\t8\nCM2\t5\t6\nZZZ\t1\t1\n")
	out, err := exec.Command("bash", "pr-autoresolve.sh", "--union", main, pr, base).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := hdr + "ALL\t12\t22\nBLB\t3\t4\nCM2\t5\t6\nCMM\t7\t8\nWOE\t1\t1\nZZZ\t1\t1\n"
	if string(out) != want {
		t.Errorf("union:\n%q\nwant\n%q", out, want)
	}
}
