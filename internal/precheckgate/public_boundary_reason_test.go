package precheckgate

// pr-precheck.sh's public-boundary step reports the first log line its reason
// pattern matches, so a failure that matches nothing prints "public boundary:"
// with an empty reason. A corpus whose cardsfolder is a symlink makes
// TestNoVerbatimForgeLines fail with "the corpus gave only N script lines",
// which the pattern must name too.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestPublicBoundaryReasonNamesScriptLines(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-precheck.sh"))
	if err != nil {
		t.Fatalf("read scripts/pr-precheck.sh: %v", err)
	}
	m := regexp.MustCompile(`fail "public boundary: \$\(grep -m1 -E '([^']+)'`).FindSubmatch(script)
	if m == nil {
		t.Fatal("scripts/pr-precheck.sh has no public-boundary reason grep")
	}
	reason, err := regexp.Compile(string(m[1]))
	if err != nil {
		t.Fatalf("public-boundary reason %q: %v", m[1], err)
	}
	for _, msg := range []string{
		"the corpus gave only 0 script lines, fewer than 50000: the scan would run vacuously",
		"a_test.go: holds a line of probe.txt verbatim; rewrite the fixture",
		"dirac/x.md: private path \".ds4/\"",
	} {
		if !reason.MatchString(msg) {
			t.Errorf("public-boundary reason %q does not match %q: the failure would print a blank reason", reason, msg)
		}
	}
}
