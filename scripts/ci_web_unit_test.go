package scripts

// The web client's unit tests (vitest), svelte-check and eslint run in CI: job web-unit in ci.yml.
// Before it, no workflow ran them and web/src/**/*.test.ts files were merged unexecuted. The job is
// advisory until it has been green on main for a week, so it is not yet in ci-ok's needs.

import (
	"strings"
	"testing"
)

func TestCIRunsTheWebUnitTests(t *testing.T) {
	ci := readText(t, "../.github/workflows/ci.yml")
	job := jobBlock(t, ci, "web-unit")
	for _, want := range []string{"npm test", "npm run check", "npm run lint"} {
		if !strings.Contains(job, want) {
			t.Errorf("job web-unit does not run %q", want)
		}
	}
	needs := ""
	for _, l := range strings.Split(jobBlock(t, ci, "ci-ok"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "needs:") {
			needs = l
			break
		}
	}
	if needs == "" {
		t.Fatal("job ci-ok has no needs: line")
	}
	if strings.Contains(needs, "web-unit") {
		t.Errorf("ci-ok's needs names web-unit; it is advisory until green on main for a week: %s", needs)
	}
}
