package scripts

// dirac/oracle/testdata/sentinels.txt is the set of oracle tests every Mergify batch runs (ci.yml,
// oracle-sentinels). A name with no func in dirac/oracle would run zero tests and read green, so the list is checked.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestSentinelsExist(t *testing.T) {
	t.Parallel()
	declared := map[string]bool{}
	decl := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
	files, err := filepath.Glob("../dirac/oracle/*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no dirac/oracle test files: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range decl.FindAllStringSubmatch(string(b), -1) {
			declared[m[1]] = true
		}
	}
	b, err := os.ReadFile("../dirac/oracle/testdata/sentinels.txt")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		n++
		if seen[name] {
			t.Errorf("sentinels.txt lists %s twice", name)
		}
		seen[name] = true
		if !declared[name] {
			t.Errorf("sentinels.txt names %s, which no dirac/oracle test declares: remove or rename it", name)
		}
	}
	if n == 0 {
		t.Fatal("sentinels.txt names no tests")
	}
}

// TestSentinelDealCoversEveryName: ci-sentinels.sh deals every sentinel to exactly one process of one shard, for the
// shards and processes ci.yml's oracle-sentinels job runs (a name dealt nowhere would never run in a batch).
func TestSentinelDealCoversEveryName(t *testing.T) {
	t.Parallel()
	ci := readText(t, "../.github/workflows/ci.yml")
	job := jobBlock(t, ci, "oracle-sentinels")
	m := regexp.MustCompile(`shard: \[([0-9, ]+)\]`).FindStringSubmatch(job)
	call := regexp.MustCompile(`ci-sentinels\.sh \$\{\{ matrix\.shard \}\} ([0-9]+) ([0-9]+)`).FindStringSubmatch(job)
	if m == nil || call == nil {
		t.Fatal("oracle-sentinels: no shard matrix or no ci-sentinels.sh <shard> <shards> <procs> call")
	}
	shards, _ := strconv.Atoi(call[1])
	if n := len(strings.Split(m[1], ",")); n != shards {
		t.Fatalf("oracle-sentinels: the matrix has %d shards, ci-sentinels.sh is told %d", n, shards)
	}
	cmd := exec.Command("bash", "scripts/ci-sentinels.sh", "--plan", call[1], call[2])
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-sentinels.sh --plan: %v\n%s", err, out)
	}
	dealt := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			t.Fatalf("ci-sentinels.sh --plan: bad line %q", line)
		}
		if k, err := strconv.Atoi(f[0]); err != nil || k < 0 || k >= shards {
			t.Errorf("%s dealt to shard %s of %d", f[3], f[0], shards)
		}
		dealt[f[3]]++
	}
	b, err := os.ReadFile("../dirac/oracle/testdata/sentinels.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if name := strings.TrimSpace(line); name != "" {
			want[name] = true
		}
	}
	for name := range want {
		if dealt[name] != 1 {
			t.Errorf("%s dealt %d times, want once", name, dealt[name])
		}
	}
	for name := range dealt {
		if !want[name] {
			t.Errorf("ci-sentinels.sh deals %s, which sentinels.txt does not name", name)
		}
	}
}
