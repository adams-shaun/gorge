package scripts

// TestAutoresolveMatcher: scripts/pr-autoresolve.sh resolves a conflict alone only when every conflicted path is a
// generated file in scripts/autoresolve-paths.tsv (--match prints the rest). A path the matcher wrongly accepts would
// be overwritten with main's side and pushed to a PR; one it wrongly rejects leaves the PR CONFLICTING.

import (
	"os/exec"
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
