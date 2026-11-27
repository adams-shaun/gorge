package scripts

// scripts/green-main.sh against a stubbed gh serving fixture pages, and scripts/image-freshness.sh against a
// stubbed inspect command with a pinned clock.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubGH answers `gh api <path> --jq <filter>` from files in $FIXTURE: runs-<page>.json for the run list,
// jobs-<id>.json for a run's jobs, filtered through jq like gh does. Every call is appended to $FIXTURE/calls.
const stubGH = `#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || { echo "stub gh: want api, got $1" >&2; exit 2; }
path=$2; filter=.
shift 2
while [ $# -gt 0 ]; do
  case $1 in --jq) filter=$2; shift 2 ;; *) shift ;; esac
done
echo "$path" >> "$FIXTURE/calls"
case $path in
  */actions/workflows/oracle.yml/runs\?*)
    page=$(printf '%s' "$path" | sed -n 's/.*[?&]page=\([0-9]*\).*/\1/p')
    f=$FIXTURE/runs-$page.json ;;
  */actions/runs/*/jobs*)
    id=$(printf '%s' "$path" | sed -n 's|.*/actions/runs/\([0-9]*\)/jobs.*|\1|p')
    f=$FIXTURE/jobs-$id.json ;;
  *) echo "stub gh: unknown path $path" >&2; exit 2 ;;
esac
[ -f "$f" ] || f=$FIXTURE/empty.json
jq -r "$filter" "$f"
`

// greenFixture writes nSkip successful runs whose oracle jobs were all skipped (newest first), then, when
// withGreen, one run whose oracle jobs succeeded. Runs are split into pages of 100.
func greenFixture(t *testing.T, nSkip int, withGreen bool) (dir string) {
	t.Helper()
	dir = t.TempDir()
	type run struct {
		id   int
		sha  string
		jobs string
	}
	var runs []run
	for i := 0; i < nSkip; i++ {
		runs = append(runs, run{1000 + i, fmt.Sprintf("skip%036d", i),
			`[{"name":"build","conclusion":"success"},{"name":"oracle (1)","conclusion":"skipped"},{"name":"oracle-bisect","conclusion":"skipped"}]`})
	}
	if withGreen {
		runs = append(runs, run{9999, "green00000000000000000000000000000000000",
			`[{"name":"build","conclusion":"success"},{"name":"oracle (1)","conclusion":"success"},{"name":"oracle-bisect","conclusion":"skipped"}]`})
	}
	for p := 0; p*100 < len(runs) || p == 0; p++ {
		var items []string
		for i := p * 100; i < len(runs) && i < (p+1)*100; i++ {
			r := runs[i]
			items = append(items, fmt.Sprintf(`{"id":%d,"head_sha":%q,"conclusion":"success"}`, r.id, r.sha))
			write(t, filepath.Join(dir, fmt.Sprintf("jobs-%d.json", r.id)), `{"jobs":`+r.jobs+`}`, 0o644)
		}
		write(t, filepath.Join(dir, fmt.Sprintf("runs-%d.json", p+1)), `{"workflow_runs":[`+strings.Join(items, ",")+`]}`, 0o644)
	}
	write(t, filepath.Join(dir, "empty.json"), `{"workflow_runs":[],"jobs":[]}`, 0o644)
	write(t, filepath.Join(dir, "gh"), stubGH, 0o755)
	return dir
}

func runScript(t *testing.T, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	c := exec.Command("bash", args...)
	c.Env = append(os.Environ(), env...)
	var o, e strings.Builder
	c.Stdout, c.Stderr = &o, &e
	err := c.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

func TestGreenMainPagesPastSkipRuns(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("no jq")
	}
	dir := greenFixture(t, 130, true)
	env := []string{"FIXTURE=" + dir, "GH=" + filepath.Join(dir, "gh"), "GITHUB_REPOSITORY=o/r"}
	out, errOut, code := runScript(t, env, "green-main.sh")
	if code != 0 || strings.TrimSpace(out) != "green00000000000000000000000000000000000" {
		t.Fatalf("want the green run's sha, exit 0; got %d stdout=%q stderr=%s", code, out, errOut)
	}
	// Precondition: the green run sits past the first page of 100, so the pick had to page.
	calls, err := os.ReadFile(filepath.Join(dir, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "per_page=100&page=2") {
		t.Fatalf("the pick never read page 2:\n%s", calls)
	}

	dir = greenFixture(t, 130, false)
	env = []string{"FIXTURE=" + dir, "GH=" + filepath.Join(dir, "gh"), "GITHUB_REPOSITORY=o/r"}
	out, errOut, code = runScript(t, env, "green-main.sh")
	if code != 3 || out != "" || !strings.Contains(errOut, "green-main: no green oracle run on main in 130 runs") {
		t.Fatalf("want exit 3 and the no-green notice; got %d stdout=%q stderr=%s", code, out, errOut)
	}
}

func TestImageFreshnessFailsWhenStale(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 27, 6, 30, 0, 0, time.UTC)
	dir := t.TempDir()
	// The stub inspect succeeds only for $WANT.
	stub := filepath.Join(dir, "inspect")
	write(t, stub, "#!/usr/bin/env bash\necho \"$1\" >> \"$CALLS\"\n[ \"$1\" = \"$WANT\" ]\n", 0o755)
	day := func(d int) string { return now.AddDate(0, 0, -d).Format("2006-01-02") }
	env := func(want, calls string) []string {
		return []string{"NOW=" + fmt.Sprint(now.Unix()), "INSPECT=" + stub, "WANT=" + want, "CALLS=" + filepath.Join(dir, calls)}
	}

	out, errOut, code := runScript(t, env("ghcr.io/o/r:"+day(1), "fresh"), "image-freshness.sh", "ghcr.io/o/r", "2")
	if code != 0 || !strings.Contains(out, "image-freshness: newest dated tag ghcr.io/o/r:2026-11-26") {
		t.Fatalf("1-day-old tag: want exit 0; got %d stdout=%q stderr=%s", code, out, errOut)
	}

	out, errOut, code = runScript(t, env("ghcr.io/o/r:"+day(5), "stale"), "image-freshness.sh", "ghcr.io/o/r", "2")
	if code != 1 || !strings.Contains(out+errOut, "image-freshness: no dated tag of ghcr.io/o/r in the last 2 days") {
		t.Fatalf("5-day-old tag, max 2: want exit 1; got %d stdout=%q stderr=%s", code, out, errOut)
	}
	// Precondition: the stale run probed today back to 2 days ago, and only those.
	calls, err := os.ReadFile(filepath.Join(dir, "stale"))
	if err != nil {
		t.Fatal(err)
	}
	want := "ghcr.io/o/r:2026-11-27\nghcr.io/o/r:2026-11-26\nghcr.io/o/r:2026-11-25\n"
	if string(calls) != want {
		t.Fatalf("probed tags:\n%s\nwant:\n%s", calls, want)
	}
}
