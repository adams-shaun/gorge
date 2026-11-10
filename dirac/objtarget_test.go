package dirac

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// objTargetAllow names each raw "< PlayerTarget0" compare that is not an
// object-target read, by file and the line's trimmed text.
var objTargetAllow = map[string]string{
	"act_mayshuffle.go":   "if p == NoTarget || p < PlayerTarget0 || p > PlayerTarget1 {",                                                                   // a player-range test
	"tgt_stackability.go": "func isStackTarget(c uint8) bool { return c >= StackTarget0 && c < PlayerTarget0 } // seat-ok: the stack-code span, not a seat", // the stack-ability range test
}

// TestObjTargetPredicate fails on any raw "x < PlayerTarget0" compare in
// the kernel's non-test sources (subpackages included) outside
// isObjTarget/IsObjTarget and objTargetAllow: since a target byte in
// StackTarget0..PlayerTarget0-1 names a stack ability, so "below the
// players" no longer means "an object".
func TestObjTargetPredicate(t *testing.T) {
	re := regexp.MustCompile(`<\s*(kernel\.)?PlayerTarget0\b`)
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !re.MatchString(line) || strings.Contains(line, "<=") {
				continue
			}
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") || objTargetAllow[filepath.Base(path)] == trim {
				continue
			}
			t.Errorf("%s:%d: raw object-target compare %q: use isObjTarget / kernel.IsObjTarget (or add it to objTargetAllow)", path, i+1, trim)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
