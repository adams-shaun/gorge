package dirac

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// landgenRow is one row of dirac/LANDGEN (the file's own comment says what each field means).
type landgenRow struct {
	line                            int
	class, path, generator, command string
}

// parseLandgen reads LANDGEN: a row is four tab-separated fields; blank lines and "#" lines are skipped.
func parseLandgen(b []byte) ([]landgenRow, error) {
	var rows []landgenRow
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("line %d: %d tab-separated fields, want 4 (class, path, generator, command)", i+1, len(f))
		}
		rows = append(rows, landgenRow{i + 1, f[0], f[1], f[2], f[3]})
	}
	return rows, nil
}

// landgenProblems says what is wrong with rows against the tree at root, and which regen rows match
// no file there. A regen file may be absent: its command creates it, and a row can land before the
// change that adds its file (dirac/testdata/layout.tsv, ticket A4). An owned file must exist.
func landgenProblems(root string, rows []landgenRow) (bad, absent []string) {
	seen := map[string]bool{}
	for _, r := range rows {
		at := fmt.Sprintf("line %d (%s)", r.line, r.path)
		if r.class != "owned" && r.class != "regen" {
			bad = append(bad, fmt.Sprintf("%s: class %q, want owned or regen", at, r.class))
		}
		if r.path == "" || path.Clean(r.path) != r.path || path.IsAbs(r.path) || strings.HasPrefix(r.path, "../") {
			bad = append(bad, at+": not a clean path relative to the repository root")
			continue
		}
		if seen[r.path] {
			bad = append(bad, at+": listed twice")
		}
		seen[r.path] = true
		if r.generator == "" || strings.TrimSpace(r.command) == "" {
			bad = append(bad, at+": no generator or no command")
		}
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(r.path)))
		if err == nil && len(files) == 0 && r.class == "regen" {
			absent = append(absent, at)
			continue
		}
		if err != nil || len(files) == 0 {
			bad = append(bad, fmt.Sprintf("%s: matches no file (%v)", at, err))
			continue
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				bad = append(bad, fmt.Sprintf("%s: %v", at, err))
				continue
			}
			switch {
			case r.generator == "-":
				if !strings.HasSuffix(f, "_test.go") && !(strings.Contains(f, "/testdata/") && strings.HasSuffix(f, ".tsv")) {
					bad = append(bad, fmt.Sprintf("%s: %s has generator \"-\", which only a Go test source of pins or a headerless testdata TSV a test rewrites may", at, f))
				}
			case !strings.Contains(string(b[:min(len(b), 2048)]), r.generator):
				bad = append(bad, fmt.Sprintf("%s: %s's header does not name its generator %q", at, f, r.generator))
			}
		}
	}
	return bad, absent
}

// TestLandgenListed holds dirac/LANDGEN, the one list of the files the landing step regenerates, to
// its format and to the tree: every row's path exists and every generated file names its generator.
func TestLandgenListed(t *testing.T) {
	b, err := os.ReadFile("LANDGEN")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseLandgen(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("dirac/LANDGEN lists no file")
	}
	bad, absent := landgenProblems("..", rows)
	for _, p := range bad {
		t.Errorf("dirac/LANDGEN %s", p)
	}
	for _, p := range absent {
		t.Logf("dirac/LANDGEN %s: a regen file not in this tree yet", p)
	}

	// The check itself, on a fixture tree.
	root := t.TempDir()
	for name, body := range map[string]string{
		"gen.md":       "Generated: TestGen rewrites this page.\n",
		"pins_test.go": "package x\n",
		"plain.md":     "hand-written\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		row  string
		good bool
	}{
		{"owned\tgen.md\tTestGen\tgo test -run TestGen", true},
		{"regen\tg*.md\tGenerated\tgo test -run TestGen", true},
		{"regen\t*.md\tGenerated\tgo test -run TestGen", false},
		{"regen\tpins_test.go\t-\tKGEN_REPIN=1 go test", true},
		{"other\tgen.md\tTestGen\tgo test", false},
		{"owned\tmissing.md\tTestGen\tgo test", false},
		{"regen\tmissing.md\tTestGen\tgo test", true},
		{"owned\tplain.md\tTestGen\tgo test", false},
		{"owned\tplain.md\t-\tgo test", false},
		{"owned\t../gen.md\tTestGen\tgo test", false},
		{"owned\tgen.md\tTestGen\t ", false},
		{"owned\tgen.md\tTestGen\tgo test\nregen\tgen.md\tTestGen\tgo test", false},
	} {
		rows, err := parseLandgen([]byte(c.row + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := landgenProblems(root, rows); (len(got) == 0) != c.good {
			t.Errorf("%q: problems %q, want good %v", c.row, got, c.good)
		}
	}
	if _, err := parseLandgen([]byte("owned\tgen.md\tTestGen\n")); err == nil {
		t.Error("a three-field row parsed")
	}
}
