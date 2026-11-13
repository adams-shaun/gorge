package scripts

// TestMakeTestOnePassesRegexDollarsVerbatim guards the test-one recipe's $(value …) use: a
// regex like ^A$|^B$ must reach the script unexpanded, not lose $| as a make automatic variable.

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMakeTestOnePassesRegexDollarsVerbatim(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("no make on PATH")
	}
	c := exec.Command("make", "-n", "-C", "..", "test-one",
		"PKG=./p", "RUN=^A$|^B$", "SKIP=^C$|^D$", "TESTENV=X=$y")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n -C .. test-one: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "bash scripts/test-targets.sh one") {
		t.Fatalf("test-one recipe did not run; output:\n%s", got)
	}
	// Precondition: with plain $(RUN), make expands these arguments and prints
	// RUN='^A^B$' (the $| is an empty automatic variable). The verbatim spellings
	// below only appear when the recipe hands the text through unexpanded.
	if strings.Contains(got, "RUN='^A^B$'") {
		t.Fatalf("RUN regex was expanded instead of passed verbatim; output:\n%s", got)
	}
	for _, want := range []string{"RUN='^A$|^B$'", "SKIP='^C$|^D$'", "TESTENV='X=$y'"} {
		if !strings.Contains(got, want) {
			t.Errorf("make -n output missing %q; output:\n%s", want, got)
		}
	}
}
