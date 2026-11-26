package testutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// longWaitSleep matches a literal time.Sleep of 5 s or more; longWaitBound
// matches a literal of 6 s or more behind a time.Now().Add(, time.After(,
// WithTimeout(..., or time.Since(...) < bound. Sleep is held to the stricter
// bar: a blocking sleep that long is never a wait whose expiry fails the
// test — it is a negative wait or a bug.
var (
	longWaitSleep = regexp.MustCompile(`time\.Sleep\(\s*(\d+)\s*\*\s*time\.(Second|Minute)\b`)
	longWaitBound = regexp.MustCompile(`(?:time\.Now\(\)\.Add\(|time\.After\(|WithTimeout\([^,]+,\s*|time\.Since\([^)]*\)\s*<\s*)\(?(\d+)\s*\*\s*time\.(Second|Minute)\b`)
)

// longWaitHit reports whether one source line holds a fixed long wait
// literal. A trailing `// wait:ok <reason>` comment exempts the line: it
// marks a negative wait ("nothing happens within N s" — its expiry is the
// success arm) or a literal that is itself a production value under test.
func longWaitHit(line string) bool {
	if strings.Contains(line, "// wait:ok") {
		return false
	}
	if c := strings.Index(line, "//"); c >= 0 {
		line = line[:c]
	}
	ge := func(re *regexp.Regexp, min int) bool {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			n, _ := strconv.Atoi(m[1])
			if m[2] == "Minute" {
				n *= 60
			}
			if n >= min {
				return true
			}
		}
		return false
	}
	return ge(longWaitSleep, 5) || ge(longWaitBound, 6)
}

// TestNoFixedLongWaitsSeeded runs the line matcher on in-memory lines, so the
// matcher's behaviour is pinned independently of the tree it walks.
func TestNoFixedLongWaitsSeeded(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"time.Sleep(5 * time.Second)", true},
		{"deadline := time.Now().Add(50 * time.Second)", true},
		{"deadline := time.Now().Add(50 * time.Second) // wait:ok negative wait", false},
		{"time.Now().Add(5 * time.Second)", false},
		// Precondition: a line already on the budget (or a short literal, or a
		// duration in a comment) is not a hit; without this the walk test
		// below could pass vacuously.
		{"case <-time.After(testutil.WaitBudget):", false},
		{"time.Sleep(4 * time.Second)", false},
		{"// deadline := time.Now().Add(50 * time.Second)", false},
	}
	for _, tc := range cases {
		if got := longWaitHit(tc.line); got != tc.want {
			t.Errorf("longWaitHit(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

// TestNoFixedLongWaitsInTests walks every test file in the module and fails
// on a line holding a fixed long wait literal without a `// wait:ok <reason>`
// comment: use testutil.WaitBudget for a wait whose expiry fails the test, or
// add // wait:ok <reason>. It is a rule with no per-file counts, so there is
// no allow-list to race.
func TestNoFixedLongWaitsInTests(t *testing.T) {
	root := filepath.Join("..", "..")
	var hits []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); p != root && strings.HasPrefix(n, ".") || n == "node_modules" || n == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		// Benchmarks are out of scope: they assert nothing on wall time.
		if !strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_bench_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(p), "../../"))
		// wait.go holds the constant itself (not a test file, listed for
		// clarity); wait_lint_test.go holds the banned lines above as raw
		// strings, so it is the lint's one exemption.
		if rel == "testkit/testutil/wait.go" || rel == "testkit/testutil/wait_lint_test.go" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if longWaitHit(line) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d test lines hold a fixed long wait literal: use testutil.WaitBudget for a wait whose expiry fails the test, or add // wait:ok <reason>.\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}
