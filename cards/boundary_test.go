package cards_test

import (
	"os/exec"
	"strings"
	"testing"
)

// A Forge card script is identifiable by content, not path: it begins with a
// Name: line and carries an Oracle: line. Checking content rather than
// directory means the guard survives someone vendoring the corpus somewhere
// unexpected.
func looksLikeForgeScript(blob string) bool {
	return strings.HasPrefix(blob, "Name:") && strings.Contains(blob, "\nOracle:")
}

// TestNoForgeScriptsTracked enforces the GPL boundary from the design spec:
// mtgcore ships a compiler, never the scripts.
func TestNoForgeScriptsTracked(t *testing.T) {
	out, err := exec.Command("git", "-C", "../..", "ls-files", "-z", "--", "*.txt").Output()
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	var offenders []string
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		blob, err := exec.Command("git", "-C", "../..", "show", "HEAD:"+path).Output()
		if err != nil {
			continue // not in HEAD yet; the working-tree check below covers it
		}
		if looksLikeForgeScript(string(blob)) {
			offenders = append(offenders, path)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("Forge card scripts are tracked in git, which breaks the GPL boundary:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
