package dirac

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// A test file's name says what it pins. It has a separator (<topic>_<what>)
// and no round suffix: a trailing _followup/_fix/_fixes/_r<N>/_s<N>/_pair<N>
// token, or a bare trailing digit (layerN excepted). The existing stems that
// break this are listed in testdata/test_name_grandfathered.txt, which only
// shrinks: a renamed or deleted file leaves it, nothing joins it.
//
// testNamesGrandfatheredMax is that list's ceiling, which is what makes the
// shrink-only promise enforceable: lower it when a file leaves the list,
// never raise it.
const testNamesGrandfatheredMax = 128

var (
	roundToken = regexp.MustCompile(`(^|_)(followup|round[0-9]*|r[0-9]+|s[0-9]+|cs[0-9]+|fix|fixes|pair[0-9]+)$`)
	digitEnd   = regexp.MustCompile(`[0-9]$`)
	layerEnd   = regexp.MustCompile(`layer[0-9]$`)
)

// badTestName returns why a test file stem breaks the naming rule, or "".
func badTestName(stem string) string {
	switch {
	case !strings.Contains(stem, "_"):
		return "has no separator"
	case roundToken.MatchString(stem):
		return "ends in a round suffix"
	case digitEnd.MatchString(stem) && !layerEnd.MatchString(stem):
		return "ends in a number"
	}
	return ""
}

func grandfathered(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("testdata/test_name_grandfathered.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if s := strings.TrimSpace(sc.Text()); s != "" {
			out = append(out, s)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTestFileNames(t *testing.T) {
	names, err := filepath.Glob("*_test.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no test files: %v", err)
	}
	old := grandfathered(t)
	if !slices.IsSorted(old) {
		t.Error("testdata/test_name_grandfathered.txt is not sorted")
	}
	if len(old) > testNamesGrandfatheredMax {
		t.Errorf("testdata/test_name_grandfathered.txt has %d entries, over the ceiling %d: the list only shrinks, lower testNamesGrandfatheredMax instead of raising it", len(old), testNamesGrandfatheredMax)
	}
	have := map[string]bool{}
	for _, n := range names {
		stem := strings.TrimSuffix(n, "_test.go")
		have[stem] = true
		if why := badTestName(stem); why != "" && !slices.Contains(old, stem) {
			t.Errorf("%s %s: name it <topic>_<what> with no round suffix; new stems are not added to the grandfathered list", n, why)
		}
	}
	for _, stem := range old {
		switch {
		case !have[stem]:
			t.Errorf("grandfathered stem %q has no file: delete its line (the list only shrinks)", stem)
		case badTestName(stem) == "":
			t.Errorf("grandfathered stem %q now follows the rule: delete its line (the list only shrinks)", stem)
		}
	}
}

// The rule itself: each way of being bad is caught, and good names pass.
func TestBadTestNameCatchesEachShape(t *testing.T) {
	for stem, want := range map[string]bool{
		"emitfix": true, "repro_fixes": true, "foo_followup": true, "foo_s41": true,
		"outcome_pair134": true, "cmdblock5": true, "foo_bar2": true, "foo_r3": true,
		"static_layer6": false, "twopiles_params": false, "std_set_repros_lockstep": false,
	} {
		if got := badTestName(stem) != ""; got != want {
			t.Errorf("badTestName(%q) flagged=%v, want %v", stem, got, want)
		}
	}
}
