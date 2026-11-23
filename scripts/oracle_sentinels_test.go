package scripts

// dirac/oracle/testdata/sentinels.txt is the set of oracle tests every Mergify batch runs (ci.yml,
// oracle-sentinels). A name with no func in dirac/oracle would run zero tests and read green, so the list is checked.

import (
	"os"
	"path/filepath"
	"regexp"
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
