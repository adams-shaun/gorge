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

// subSecondMs matches an integer-literal sub-second millisecond duration:
// `200 * time.Millisecond`, `30*time.Millisecond`.
var subSecondMs = regexp.MustCompile(`\b(\d{1,3})\s*\*\s*time\.Millisecond\b`)

// subSecondAllow pins, per test file, how many sub-second `N * time.Millisecond`
// literals it may hold. A test whose PASS/FAIL depends on such a short
// wall-clock window goes red on a loaded box with no bad commit (2026-10-12:
// TestLedgerMarksCaretaker, 200ms, lost the race at load ~100). The entries
// below were classified when this ratchet landed: each is a poll sleep, a
// "nothing happens in this window" negative wait (a longer wait only makes it
// stronger), a fake-clock value, or a production-config value under test.
//
// The map only shrinks: raising a count, or adding a file, fails. Prefer a
// channel/hook, a poll with a generous ceiling, or a deadline >= 10x the
// p99 under load, with a comment saying why.
var subSecondAllow = map[string]int{
	"engine/cards/cache_lock_flock_test.go":               2,
	"server/gorged/artbounds_test.go":                     5,
	"server/gorged/artfill_test.go":                       2,
	"server/gorged/artprewarm_test.go":                    2,
	"server/gorged/artwait_test.go":                       3,
	"server/gorged/main_test.go":                          12,
	"server/gorged/manabrew_cardtext_test.go":             4,
	"server/gorged/manabrew_test.go":                      1,
	"server/host/caretaker_test.go":                       6,
	"server/host/httpapi/rest_test.go":                    1,
	"server/host/httpapi/sse_test.go":                     6,
	"server/host/humanseat_test.go":                       2,
	"server/host/manabrewhttp/gameover_reconnect_test.go": 5,
	"server/host/manabrewhttp/golden_test.go":             1,
	"server/host/manabrewhttp/manabrewhttp_test.go":       2,
	"server/host/manabrewhttp/parity_test.go":             1,
	"server/host/manabrewhttp/synthetic_stream_test.go":   2,
	"server/host/publishanswer_test.go":                   2,
	"server/host/slots_test.go":                           2,
	"server/host/stall_test.go":                           1,
}

func TestNoNewSubSecondWallClockThresholdInTests(t *testing.T) {
	root := filepath.Join("..", "..")
	hits := map[string][]string{}
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
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(p), "../../"))
		for i, line := range strings.Split(string(b), "\n") {
			if c := strings.Index(line, "//"); c >= 0 {
				line = line[:c]
			}
			for _, m := range subSecondMs.FindAllStringSubmatch(line, -1) {
				if n, _ := strconv.Atoi(m[1]); n < 1000 {
					hits[rel] = append(hits[rel], fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(hits))
	for f := range hits {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		allow := subSecondAllow[f]
		if len(hits[f]) > allow {
			t.Errorf("%s holds %d sub-second `N * time.Millisecond` literals, allowlist %d: a PASS/FAIL that depends on a short wall-clock window goes red on a loaded box.\n"+
				"Use a channel/hook, a poll with a >=10s ceiling, or a deadline >=10x the p99 under load.\n  %s",
				f, len(hits[f]), allow, strings.Join(hits[f], "\n  "))
		} else if len(hits[f]) < allow {
			t.Errorf("%s now holds %d literals, allowlist says %d: lower subSecondAllow (it only shrinks)", f, len(hits[f]), allow)
		}
	}
	for f, allow := range subSecondAllow {
		if _, ok := hits[f]; !ok {
			t.Errorf("%s allowlisted for %d but has none: delete the entry", f, allow)
		}
	}
}
