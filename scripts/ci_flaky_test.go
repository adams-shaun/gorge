package scripts

// The flaky-test quarantine: scripts/flaky.tsv lists tests that run on every merge-queue batch but cannot fail it
// (scripts/ci-flaky.sh, scripts/ci-test-shard.sh, ci.yml's test job). A global -skip matches by test name in every
// package of a shard, so the list's format is held here.

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	flakyModule  = "github.com/adams-shaun/gorge"
	flakyMaxRows = 20
	// flakyGolden is the golden chain heads test: it gates every batch in a step of its own.
	flakyGolden = "TestHeads"
)

var flakyTestName = regexp.MustCompile(`^Test[A-Za-z0-9_]+$`)

// testDecls maps every top-level Test function to the directories (relative to root, one entry per declaring
// _test.go file) that declare it.
func testDecls(root string) (map[string][]string, error) {
	decl := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Hidden directories (.git, .cards, nested worktrees, scratch copies) hold no package of this module.
			if n := d.Name(); path != root && (strings.HasPrefix(n, ".") || n == "node_modules" || n == "web") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, m := range decl.FindAllSubmatch(b, -1) {
			out[string(m[1])] = append(out[string(m[1])], filepath.ToSlash(dir))
		}
		return nil
	})
	return out, err
}

// flakyListErrors returns every rule the list text breaks, one error per violation.
func flakyListErrors(root, list string, decls map[string][]string) []error {
	var errs []error
	rows := 0
	seen := map[string]int{}
	for i, line := range strings.Split(list, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows++
		bad := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("line %d: "+format, append([]any{i + 1}, a...)...))
		}
		f := strings.Split(line, "\t")
		if len(f) != 5 || slices.ContainsFunc(f, func(s string) bool { return strings.TrimSpace(s) == "" }) {
			bad("want 5 non-empty tab-separated fields (package, test, ticket, added, signature), got %q", line)
			continue
		}
		pkg, test, added := f[0], f[1], f[3]
		if _, err := time.Parse("2006-01-02", added); err != nil {
			bad("added %q is not a YYYY-MM-DD date", added)
		}
		if !flakyTestName.MatchString(test) {
			bad("test %q is not one top-level test name", test)
		}
		if test == flakyGolden {
			bad("%s is never listed: the golden chain heads always gate", flakyGolden)
		}
		if first, ok := seen[test]; ok {
			bad("test %s already listed on line %d", test, first)
		}
		seen[test] = i + 1
		rel, inModule := strings.CutPrefix(pkg, flakyModule+"/")
		st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if !inModule || err != nil || !st.IsDir() {
			bad("package %q is not a directory of module %s", pkg, flakyModule)
			continue
		}
		if dirs := decls[test]; len(dirs) != 1 || dirs[0] != rel {
			bad("test %s must be declared by exactly one _test.go file, in %s; declared in %v", test, rel, dirs)
		}
	}
	if rows > flakyMaxRows {
		errs = append(errs, fmt.Errorf("%d rows, at most %d: fix flaky tests rather than list them", rows, flakyMaxRows))
	}
	return errs
}

func TestFlakyListWellFormed(t *testing.T) {
	t.Parallel()
	const root = ".."
	decls, err := testDecls(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(decls["TestFlakyListWellFormed"]) != 1 {
		t.Fatalf("the declaration walk did not find this test once: %v", decls["TestFlakyListWellFormed"])
	}

	t.Run("committed", func(t *testing.T) {
		list := readText(t, "flaky.tsv")
		if !strings.HasPrefix(list, "#") {
			t.Error("flaky.tsv lost its header comment")
		}
		if err := errors.Join(flakyListErrors(root, list, decls)...); err != nil {
			t.Errorf("flaky.tsv:\n%v", err)
		}
	})

	row := func(pkg, test string) string {
		return flakyModule + "/" + pkg + "\t" + test + "\tst-x\t2026-11-28\ttimed out\n"
	}
	// unique: names declared by exactly one file, under scripts/, so each of their rows is good on its own.
	var unique, twice []string
	for name, dirs := range decls {
		switch {
		case len(dirs) == 1 && dirs[0] == "scripts":
			unique = append(unique, name)
		case slices.ContainsFunc(dirs, func(d string) bool { return d != dirs[0] }):
			twice = append(twice, name)
		}
	}
	slices.Sort(unique)
	slices.Sort(twice)
	if len(unique) <= flakyMaxRows || len(twice) == 0 {
		t.Fatalf("fixtures need over %d unique names in scripts (%d) and a name in two packages (%d)", flakyMaxRows, len(unique), len(twice))
	}
	var full strings.Builder
	for _, name := range unique[:flakyMaxRows] {
		full.WriteString(row("scripts", name))
	}
	good := "# header\n" + full.String()
	if err := errors.Join(flakyListErrors(root, good, decls)...); err != nil {
		t.Fatalf("a good list of %d rows was rejected:\n%v", flakyMaxRows, err)
	}
	// The golden row is judged as if its name were declared once, so only the never-listed rule can reject it.
	if !slices.Contains(decls[flakyGolden], "engine/rules") {
		t.Fatalf("%s is not declared in engine/rules: %v", flakyGolden, decls[flakyGolden])
	}
	onlyGolden := maps.Clone(decls)
	onlyGolden[flakyGolden] = []string{"engine/rules"}

	// Each bad list breaks one rule, and the error must be that rule's.
	for _, tc := range []struct{ name, list, want string }{
		{"duplicate name", row("scripts", unique[0]) + row("scripts", unique[0]), "already listed on line 1"},
		{"name in two packages", row(decls[twice[0]][0], twice[0]), "exactly one _test.go file"},
		{"unknown package", row("no/such/package", unique[0]), "is not a directory of module"},
		{"21 rows", full.String() + row("scripts", unique[flakyMaxRows]), "21 rows, at most 20"},
		{"golden heads listed", row("engine/rules", flakyGolden), "is never listed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "flaky.tsv")
			if err := os.WriteFile(path, []byte(tc.list), 0o644); err != nil {
				t.Fatal(err)
			}
			errs := flakyListErrors(root, readText(t, path), onlyGolden)
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tc.want) {
				t.Errorf("want exactly one error containing %q, got %v", tc.want, errs)
			}
		})
	}
}

// writeExec writes an executable shell script.
func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// runFlaky runs ci-flaky.sh with the list text as FLAKY_FILE and returns its output and exit code.
func runFlaky(t *testing.T, list string, env []string, args ...string) (string, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flaky.tsv")
	if err := os.WriteFile(path, []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{"ci-flaky.sh"}, args...)...)
	cmd.Env = append(append(os.Environ(), "FLAKY_FILE="+path), env...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	return string(out), cmd.ProcessState.ExitCode()
}

func TestFlakyScriptNeverFails(t *testing.T) {
	t.Parallel()
	const pa, pb = flakyModule + "/a", flakyModule + "/b"
	two := "# header\n" + pa + "\tTestA\tst-a\t2026-11-28\ttimed out\n" + pb + "\tTestB\tst-b\t2026-11-28\tdeadlock\n"
	// stubGo puts a `go` on PATH that records its arguments, prints 40 lines and exits rc.
	stubGo := func(t *testing.T, rc int) (env []string, calls func() string) {
		dir := t.TempDir()
		log := filepath.Join(dir, "calls")
		writeExec(t, filepath.Join(dir, "go"), fmt.Sprintf(
			"echo \"$*\" >>%q\nfor i in $(seq 1 40); do echo \"line $i end\"; done\nexit %d\n", log, rc))
		env = []string{"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"), "FLAKY_SHARD_CMD=printf '%s#B\\n' " + pa}
		return env, func() string {
			b, err := os.ReadFile(log)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	wantCall := "test -count=1 -timeout 10m -run ^TestA$ " + pa + "\n"

	t.Run("failing test", func(t *testing.T) {
		t.Parallel()
		env, calls := stubGo(t, 1)
		out, rc := runFlaky(t, two, env, "run", "0", "6")
		if rc != 0 {
			t.Errorf("exit %d, want 0:\n%s", rc, out)
		}
		if !strings.Contains(out, "QUARANTINE FAIL "+pa+" TestA\n") {
			t.Errorf("no QUARANTINE FAIL line:\n%s", out)
		}
		// The last 30 of the 40 output lines.
		if !strings.Contains(out, "line 11 end") || !strings.Contains(out, "line 40 end") || strings.Contains(out, "line 10 end") {
			t.Errorf("want the last 30 lines of the test's output:\n%s", out)
		}
		// Only the row whose package is on the shard runs (the shard's entry carries a #label).
		if got := calls(); got != wantCall {
			t.Errorf("go calls = %q, want %q", got, wantCall)
		}
	})

	t.Run("passing test", func(t *testing.T) {
		t.Parallel()
		env, calls := stubGo(t, 0)
		out, rc := runFlaky(t, two, env, "run", "0", "6")
		if rc != 0 || out != "QUARANTINE pass "+pa+" TestA\n" {
			t.Errorf("exit %d, output %q, want only the pass line", rc, out)
		}
		if got := calls(); got != wantCall {
			t.Errorf("go calls = %q, want %q", got, wantCall)
		}
	})

	t.Run("failing shard deal", func(t *testing.T) {
		t.Parallel()
		env, calls := stubGo(t, 1)
		out, rc := runFlaky(t, two, append(env, "FLAKY_SHARD_CMD=false"), "run", "0", "6")
		if rc != 0 || !strings.Contains(out, "::warning::") {
			t.Errorf("exit %d (want 0), want a warning:\n%s", rc, out)
		}
		if got := calls(); got != "" {
			t.Errorf("go ran with no shard: %q", got)
		}
	})

	for _, tc := range []struct{ name, list, want string }{
		{"skip-regex two rows", two, "TestA|TestB\n"},
		{"skip-regex empty list", "# header\n", ""},
		{"skip-regex committed list", readText(t, "flaky.tsv"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, rc := runFlaky(t, tc.list, nil, "skip-regex")
			if rc != 0 || out != tc.want {
				t.Errorf("exit %d, output %q, want %q", rc, out, tc.want)
			}
		})
	}
}

// shardCalls runs ci-test-shard.sh in a scratch root with the real ci-splits.tsv, a stub ci_shard.sh dealing every
// kind of entry and a stub go, and returns the argument line of every go call, sorted.
func shardCalls(t *testing.T, quarantine, list string) []string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts")
	bin := filepath.Join(root, "bin")
	for _, d := range []string{dir, bin} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"ci-test-shard.sh", "ci-flaky.sh", "ci-splits.tsv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(readText(t, name)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "flaky.tsv"), []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	writeExec(t, filepath.Join(dir, "ci_shard.sh"), fmt.Sprintf(
		"printf '%%s\\n' '%[1]s/dirac/kgen' '%[1]s/dirac/kgen#A' '%[1]s/engine/rules' '%[1]s/engine/rules#A' '%[1]s/decision'\n", flakyModule))
	log := filepath.Join(root, "calls")
	writeExec(t, filepath.Join(bin, "go"), fmt.Sprintf("echo \"$*\" >>%q\n", log))
	cmd := exec.Command("bash", "scripts/ci-test-shard.sh", "0", "6")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GITHUB_EVENT_NAME=pull_request", "QUARANTINE="+quarantine, "FLAKY_FILE=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ci-test-shard.sh: %v\n%s", err, out)
	}
	got := strings.Split(strings.TrimSuffix(readText(t, log), "\n"), "\n")
	slices.Sort(got)
	return got
}

func TestFlakyStepWired(t *testing.T) {
	t.Parallel()
	ci := readText(t, "../.github/workflows/ci.yml")
	const step = "Quarantined tests (cannot fail the batch)"
	lines := strings.Split(ci, "\n")
	at := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "- name: "+step) })
	if at < 0 {
		t.Fatalf("ci.yml has no step %q", step)
	}
	if !strings.Contains(strings.Join(lines[at+1:min(at+7, len(lines))], "\n"), "continue-on-error: true") {
		t.Errorf("step %q is not followed within 6 lines by continue-on-error: true", step)
	}
	// The -skip expressions live in ci-test-shard.sh, which the test job's "Test shard" step runs.
	shard := readText(t, "ci-test-shard.sh")
	if !strings.Contains(shard, "ci-flaky.sh skip-regex") {
		t.Error("ci-test-shard.sh does not run ci-flaky.sh skip-regex")
	}
	if strings.Contains(shard, "'^(TestHeads)$'") {
		t.Error("ci-test-shard.sh has a -skip expression without the quarantined names")
	}

	// The shard step's switch and the quarantine step's condition are the same batch condition.
	const batch = "github.event_name == 'pull_request' && startsWith(github.head_ref, 'mergify/merge-queue/')"
	job := jobBlock(t, ci, "test")
	for _, want := range []string{
		"QUARANTINE: ${{ " + batch + " && '1' || '' }}",
		"if: ${{ " + batch + " }}",
		`run: bash scripts/ci-flaky.sh run ${{ matrix.shard }} "$SHARDS"`,
	} {
		if !strings.Contains(job, want) {
			t.Errorf("job test lacks %q", want)
		}
	}
	shardAt := strings.Index(job, "- name: Test shard ")
	quarantineAt := strings.Index(job, "- name: "+step)
	if shardAt < 0 || quarantineAt < shardAt || strings.Count(job[shardAt:quarantineAt], "- name: ") != 1 {
		t.Errorf("step %q must directly follow the Test shard step", step)
	}

	// What go is given. Without quarantined names the calls are the gating ones: -skip on the whole and rest calls
	// only, naming the golden heads test and the split rows.
	const two = "# h\np\tTestA\tt\t2026-11-28\ts\np\tTestB\tt\t2026-11-28\ts\n"
	plain := shardCalls(t, "", "# h\n")
	count := func(match func(string) bool) (n int) {
		for _, c := range plain {
			if match(c) {
				n++
			}
		}
		return n
	}
	whole := count(func(c string) bool { return strings.Contains(c, " -skip ^(TestHeads)$|") })
	rest := count(func(c string) bool { return strings.Contains(c, " -skip ^(TestHeads)$ "+flakyModule+"/decision") })
	split := count(func(c string) bool { return strings.Contains(c, " -run ") && !strings.Contains(c, " -skip ") })
	if len(plain) != 5 || whole != 2 || rest != 1 || split != 2 {
		t.Fatalf("the stub shard must reach 2 whole, 1 rest and 2 split calls, got %d/%d/%d in:\n%s", whole, rest, split, strings.Join(plain, "\n"))
	}
	for what, got := range map[string][]string{
		"batch, empty list":         shardCalls(t, "1", "# h\n"),
		"not a batch, two-row list": shardCalls(t, "", two),
	} {
		if !slices.Equal(got, plain) {
			t.Errorf("%s: go calls =\n%s\nwant\n%s", what, strings.Join(got, "\n"), strings.Join(plain, "\n"))
		}
	}
	// A batch with a list: every call leaves the listed names out, a split row's -run call included.
	want := make([]string, len(plain))
	for i, c := range plain {
		if strings.Contains(c, " -run ") {
			at := strings.LastIndex(c, " ")
			want[i] = c[:at] + " -skip ^(TestA|TestB)$" + c[at:]
		} else {
			want[i] = strings.Replace(c, "^(TestHeads)$", "^(TestHeads|TestA|TestB)$", 1)
		}
	}
	slices.Sort(want)
	if got := shardCalls(t, "1", two); !slices.Equal(got, want) {
		t.Errorf("batch, two-row list: go calls =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
