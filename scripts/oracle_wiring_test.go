package scripts

// The oracle guards' wiring (G1 sentinels, G3 cadence and auto-bisect), read as text from ci.yml, .mergify.yml and
// oracle.yml: a guard that is not wired in, or that opens a non-draft PR, fails here rather than on main.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jobBlock returns the text of the top-level job `name` in a workflow file.
func jobBlock(t *testing.T, yml, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?ms)^  ` + regexp.QuoteMeta(name) + `:\n.*?(?:^  [A-Za-z0-9_-]+:\n|\z)`).FindString(yml)
	if m == "" {
		t.Fatalf("no job %q", name)
	}
	return m
}

func TestOracleSentinelsJobIsWired(t *testing.T) {
	t.Parallel()
	ci := readText(t, "../.github/workflows/ci.yml")
	job := jobBlock(t, ci, "oracle-sentinels")
	// Runs only on batches: never on a plain pull request, as dirac-repros.
	if !strings.Contains(job, "startsWith(github.head_ref, 'mergify/merge-queue/')") || !strings.Contains(job, "github.event_name != 'pull_request'") {
		t.Error("oracle-sentinels must run only on queue batches (the dirac-repros condition)")
	}
	if !strings.Contains(job, "dirac/oracle/testdata/sentinels.txt") || !strings.Contains(job, "./dirac/oracle") {
		t.Error("oracle-sentinels must read sentinels.txt and run ./dirac/oracle")
	}
	for _, want := range []string{"make dirac-tables", "ci-corpus.sh", "Refuse to run without the corpus"} {
		if !strings.Contains(job, want) {
			t.Errorf("oracle-sentinels lacks %q (corpus and tables prep, as dirac-repros)", want)
		}
	}
	// It must fail rather than pass with zero or skipped tests.
	if !strings.Contains(job, "go test -v") || !strings.Contains(job, "--- PASS") || !strings.Contains(job, "exit 1") {
		t.Error("oracle-sentinels must count --- PASS lines and fail when fewer than the named sentinels passed")
	}
	// Mergify merges on ci-ok alone, so ci-ok is where the job is required.
	if !strings.Contains(readText(t, "../.mergify.yml"), "check-success=ci-ok") {
		t.Error(".mergify.yml no longer requires ci-ok")
	}
	ciOK := jobBlock(t, ci, "ci-ok")
	if strings.Contains(ciOK, "dirac-repros") {
		needs := regexp.MustCompile(`needs: \[[^\]]*\]`).FindString(ciOK)
		if !strings.Contains(needs, "oracle-sentinels") {
			t.Errorf("ci-ok needs dirac-repros but not oracle-sentinels: %s", needs)
		}
		if !strings.Contains(ciOK, `needs.oracle-sentinels.result }}" = "$slices"`) {
			t.Error("ci-ok does not check oracle-sentinels' result")
		}
	}
}

func TestOracleWorkflowCadenceAndAutoRevert(t *testing.T) {
	t.Parallel()
	or := readText(t, "../.github/workflows/oracle.yml")
	if !strings.Contains(or, "-ge 6300 ") || strings.Contains(or, "19800") {
		t.Error("oracle.yml's due window must be 6300 s (1h45m)")
	}
	if !strings.Contains(or, "cron: '23 * * * *'") {
		t.Error("oracle.yml must keep its hourly cron")
	}
	job := jobBlock(t, or, "bisect")
	if !strings.Contains(job, "github.ref == 'refs/heads/main'") || !strings.Contains(job, "needs.oracle.result == 'failure'") {
		t.Error("bisect must run only on main, after a failed oracle")
	}
	if !strings.Contains(job, "scripts/oracle-bisect.sh") {
		t.Error("bisect must run scripts/oracle-bisect.sh")
	}
	if !strings.Contains(job, "gh pr create --draft") {
		t.Error("the auto-revert must open its PR with gh pr create --draft")
	}
	if !strings.Contains(job, `hotfix/auto-revert-$PR`) {
		t.Error("the auto-revert branch must be hotfix/auto-revert-<pr>")
	}
	if !strings.Contains(job, "contents: write") || !strings.Contains(job, "pull-requests: write") {
		t.Error("bisect needs contents: write and pull-requests: write")
	}
	// Write permissions on that job only, and nothing may push to main.
	if strings.Count(or, "contents: write") != 1 {
		t.Error("contents: write must appear on the bisect job only")
	}
	if regexp.MustCompile(`git push[^\n]*\bmain\b`).MatchString(or) || strings.Contains(or, "gh pr merge") || strings.Contains(or, "--ready") {
		t.Error("the bisect job must never push to main, merge or mark a PR ready")
	}
	if !strings.HasSuffix(strings.TrimSpace(job), "gh pr create --draft --base main --head \"$br\" --title \"revert: #$PR reddened the oracle (auto-bisect)\" --body-file body.md") {
		t.Error("the draft PR creation must be the last step")
	}
	if !strings.Contains(job, "echo \"---\"\n            echo \"This Robot (gorge agentctl swarm) Shan't Be Blamed For Stupid Humans\"\n          } > body.md") {
		t.Error("the PR body must end with the signature")
	}
}
