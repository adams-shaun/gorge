package conformance

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	matrixScriptRow = regexp.MustCompile(`(?m)^row (\d+) (\S+) '([a-z ]+)' `)
	matrixPageRow   = regexp.MustCompile(`(?m)^\| (\d+) \| ([^|]+) \|.*\| ([a-z ]+) \|$`)
)

func matrixRows(t *testing.T, path string, row *regexp.Regexp) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(string(b), -1) {
		if _, dup := rows[m[1]]; dup {
			t.Errorf("%s: cell %s twice", path, m[1])
		}
		rows[m[1]] = strings.ReplaceAll(m[2], " ", "") + " in " + m[3]
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no cells", path)
	}
	return rows
}

func TestWireMatrixPageMatchesScript(t *testing.T) {
	script := matrixRows(t, "../../scripts/wire-matrix.sh", matrixScriptRow)
	page := matrixRows(t, "../../docs/site/architecture/wires.md", matrixPageRow)
	for _, cell := range slices.Sorted(maps.Keys(script)) {
		if got, ok := page[cell]; !ok {
			t.Errorf("cell %s (%s) is in the script and not on the page", cell, script[cell])
		} else if got != script[cell] {
			t.Errorf("cell %s: the script says %s, the page says %s", cell, script[cell], got)
		}
	}
	for _, cell := range slices.Sorted(maps.Keys(page)) {
		if _, ok := script[cell]; !ok {
			t.Errorf("cell %s (%s) is on the page and not in the script", cell, page[cell])
		}
	}
}
