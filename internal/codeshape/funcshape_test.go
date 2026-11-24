package codeshape

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// funcCeilingsFile holds the per-function ceilings (its header says how to read and lower it).
const funcCeilingsFile = "testdata/func_ceilings.tsv"

// funcCeiling is one row of funcCeilingsFile.
type funcCeiling struct{ lines, complexity int }

// parseFuncCeilings reads funcCeilingsFile: the "#" header, then key<TAB>lines<TAB>complexity rows.
func parseFuncCeilings(b []byte) (header []string, rows map[string]funcCeiling, err error) {
	rows = map[string]funcCeiling{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			header = append(header, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, nil, fmt.Errorf("line %d: %d tab-separated fields, want 3 (key, lines, complexity)", n, len(f))
		}
		l, err1 := strconv.Atoi(f[1])
		c, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			return nil, nil, fmt.Errorf("line %d: lines %q and complexity %q must be integers", n, f[1], f[2])
		}
		if _, dup := rows[f[0]]; dup {
			return nil, nil, fmt.Errorf("line %d: %q listed twice", n, f[0])
		}
		rows[f[0]] = funcCeiling{l, c}
	}
	return header, rows, sc.Err()
}

// formatFuncCeilings writes the file back: the header, then the rows sorted by key.
func formatFuncCeilings(header []string, rows map[string]funcCeiling) []byte {
	var b bytes.Buffer
	for _, h := range header {
		b.WriteString(h + "\n")
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s\t%d\t%d\n", k, rows[k].lines, rows[k].complexity)
	}
	return b.Bytes()
}

// shapesByKey folds the measured functions by key, keeping the larger of two that share one (a name
// declared in two build-tagged files of one package).
func shapesByKey(shapes []FuncShape) (map[string]FuncShape, []string) {
	by := map[string]FuncShape{}
	var keys []string
	for _, s := range shapes {
		k := s.Key()
		old, seen := by[k]
		if !seen {
			keys = append(keys, k)
		}
		if !seen || s.Lines > old.Lines || s.Complexity > old.Complexity {
			s.Lines, s.Complexity = max(s.Lines, old.Lines), max(s.Complexity, old.Complexity)
			by[k] = s
		}
	}
	sort.Strings(keys)
	return by, keys
}

// funcMetric is one of the two per-function measures the ceilings hold.
type funcMetric struct {
	name      string // "lines" or "complexity"
	threshold int    // an unlisted function may reach it
	of        func(funcCeiling) int
	measure   func(FuncShape) int
	advice    string
}

var (
	linesMetric = funcMetric{"lines", LongFuncLines,
		func(c funcCeiling) int { return c.lines }, func(s FuncShape) int { return s.Lines },
		"Extract the new logic into a named helper or a registered handler instead of growing the function."}
	complexityMetric = funcMetric{"complexity", ComplexFuncThreshold,
		func(c funcCeiling) int { return c.complexity }, func(s FuncShape) int { return s.Complexity },
		"Move the new branch into a handler (a table entry, a per-op function registered in init, or a " +
			"helper) instead of another case or condition in the function."}
)

// funcShapeCheck holds the measured functions to the ceilings on one metric. Only growth beyond the
// slack fails: a listed function more than FuncSlack above its row, or an unlisted one more than
// FuncSlack above the metric's threshold; growth within the slack is reported in within and passes.
// A listed function below its row, or gone from the measurement, is logged in shrank.
func funcShapeCheck(shapes []FuncShape, rows map[string]funcCeiling, m funcMetric) (grew, within, shrank []string) {
	by, keys := shapesByKey(shapes)
	for _, k := range keys {
		s := by[k]
		got := m.measure(s)
		row, listed := rows[k]
		limit := m.threshold
		if listed {
			limit = max(limit, m.of(row))
		}
		slack := FuncSlack(limit)
		switch {
		case got > limit+slack && listed:
			grew = append(grew, fmt.Sprintf("%s (%s:%d) grew to %s %d, above its ceiling of %d plus its slack of %d. "+
				"%s Do not raise its row in %s.", k, s.File, s.Line, m.name, got, limit, slack, m.advice, funcCeilingsFile))
		case got > limit+slack:
			grew = append(grew, fmt.Sprintf("%s (%s:%d) is a new function over %s %d plus its slack of %d (%d). %s Do "+
				"not add it to %s; if it is a listed function renamed or moved to another package, rename its row in "+
				"the same commit without raising it.", k, s.File, s.Line, m.name, m.threshold, slack, got, m.advice,
				funcCeilingsFile))
		case got > limit:
			within = append(within, fmt.Sprintf("WARNING %s (%s:%d) is at %s %d, above its ceiling of %d but within its "+
				"slack of %d: passing. Do not raise or add its row in %s. %s", k, s.File, s.Line, m.name, got, limit,
				slack, funcCeilingsFile, m.advice))
		case listed && m.of(row) > m.threshold && got < m.of(row):
			shrank = append(shrank, fmt.Sprintf("%s shrank to %s %d (ceiling %d): lower its row to %d, or run "+
				"CODESHAPE_LOWER=1 go test -run '^TestFuncCeilingsLowerOnly$' ./internal/codeshape.",
				k, m.name, got, m.of(row), got))
		}
	}
	gone := make([]string, 0)
	for k, row := range rows {
		if _, ok := by[k]; !ok && m.of(row) > m.threshold {
			gone = append(gone, k)
		}
	}
	sort.Strings(gone)
	for _, k := range gone {
		shrank = append(shrank, fmt.Sprintf("%s is no longer over %s %d (or was renamed or moved): delete its "+
			"row, or run CODESHAPE_LOWER=1 go test -run '^TestFuncCeilingsLowerOnly$' ./internal/codeshape.",
			k, m.name, m.threshold))
	}
	return grew, within, shrank
}

// lowerFuncCeilings returns rows lowered to the measurement: each value falls to the measured one, a
// row whose function is gone or under both thresholds is dropped, and nothing is raised or added.
func lowerFuncCeilings(shapes []FuncShape, rows map[string]funcCeiling) map[string]funcCeiling {
	by, _ := shapesByKey(shapes)
	out := map[string]funcCeiling{}
	for k, row := range rows {
		s, ok := by[k]
		if !ok || (s.Lines <= LongFuncLines && s.Complexity <= ComplexFuncThreshold) {
			continue
		}
		out[k] = funcCeiling{min(row.lines, s.Lines), min(row.complexity, s.Complexity)}
	}
	return out
}

func measureFuncShapes(t *testing.T) ([]FuncShape, []string, map[string]funcCeiling) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := MeasureFuncShapes(root, ShapeDirs)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(funcCeilingsFile)
	if err != nil {
		t.Fatal(err)
	}
	header, rows, err := parseFuncCeilings(b)
	if err != nil {
		t.Fatalf("%s: %v", funcCeilingsFile, err)
	}
	return shapes, header, rows
}

func runFuncShapeRatchet(t *testing.T, m funcMetric) {
	shapes, _, rows := measureFuncShapes(t)
	grew, within, shrank := funcShapeCheck(shapes, rows, m)
	for _, s := range append(within, shrank...) {
		t.Log(s)
	}
	for _, g := range grew {
		t.Error(g)
	}
}

// TestLongFunctionsOnlyShrink holds every function in ShapeDirs to its length: a listed function
// may not grow more than FuncSlack past its row in funcCeilingsFile, and no other may pass
// LongFuncLines by more than FuncSlack. Growth within the slack and shrinking only log, so a
// one-line overrun and concurrent refactors never fail it.
func TestLongFunctionsOnlyShrink(t *testing.T) { runFuncShapeRatchet(t, linesMetric) }

// TestComplexFunctionsOnlyShrink is the same ratchet on cyclomatic complexity: the giant dispatch
// switches (kgen's instr, lowerTriggered, api, lowerKeywords; dirac's prepareGen) cannot gain more
// than FuncSlack cases, and no other function may pass ComplexFuncThreshold by more than FuncSlack.
func TestComplexFunctionsOnlyShrink(t *testing.T) { runFuncShapeRatchet(t, complexityMetric) }

// TestFuncCeilingsLowerOnly checks the ceilings file is sorted and well formed and that the ratchet
// logic fails only on growth beyond the slack. With CODESHAPE_LOWER=1 it rewrites the file lowered to the
// measurement, never raising or adding a row.
func TestFuncCeilingsLowerOnly(t *testing.T) {
	shapes, header, rows := measureFuncShapes(t)
	if os.Getenv("CODESHAPE_LOWER") == "1" {
		low := lowerFuncCeilings(shapes, rows)
		if err := os.WriteFile(funcCeilingsFile, formatFuncCeilings(header, low), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d rows lowered to the measurement (%d dropped)", funcCeilingsFile, len(low), len(rows)-len(low))
		return
	}
	b, _ := os.ReadFile(funcCeilingsFile)
	if got := formatFuncCeilings(header, rows); !bytes.Equal(got, b) {
		t.Errorf("%s is not in canonical form (header, then rows sorted by key, one tab between fields); "+
			"CODESHAPE_LOWER=1 rewrites it", funcCeilingsFile)
	}

	// The logic, on a fixture.
	fix := map[string]funcCeiling{"p big": {400, 80}, "p gone": {350, 60}, "p tame": {120, 70}}
	at := func(name string, lines, cx int) FuncShape {
		return FuncShape{Name: name, File: "p/f.go", Line: 1, Lines: lines, Complexity: cx}
	}
	// Slack: FuncSlack(400) = 12 lines, FuncSlack(80) = 5 complexity, FuncSlack(LongFuncLines) = 9 lines,
	// FuncSlack(ComplexFuncThreshold) = 5 complexity.
	ls, cs := FuncSlack(400), FuncSlack(80)
	lt, ct := LongFuncLines+FuncSlack(LongFuncLines), ComplexFuncThreshold+FuncSlack(ComplexFuncThreshold)
	for _, c := range []struct {
		name                 string
		shapes               []FuncShape
		m                    funcMetric
		grew, within, shrank int
	}{
		{"at ceiling", []FuncShape{at("big", 400, 80), at("tame", 120, 70)}, linesMetric, 0, 0, 1},
		{"grew within the slack", []FuncShape{at("big", 401, 80)}, linesMetric, 0, 1, 1},
		{"grew to the slack", []FuncShape{at("big", 400+ls, 80)}, linesMetric, 0, 1, 1},
		{"grew beyond the slack", []FuncShape{at("big", 400+ls+1, 80)}, linesMetric, 1, 0, 1},
		{"shrank", []FuncShape{at("big", 390, 80)}, linesMetric, 0, 0, 2},
		{"complexity grew within the slack", []FuncShape{at("big", 400, 80+cs)}, complexityMetric, 0, 1, 2},
		{"complexity grew beyond the slack", []FuncShape{at("big", 400, 80+cs+1)}, complexityMetric, 1, 0, 2},
		{"listed under the line threshold grows below it", []FuncShape{at("tame", 299, 70)}, linesMetric, 0, 0, 2},
		{"listed under the line threshold crosses it within the slack", []FuncShape{at("tame", 301, 70)}, linesMetric, 0, 1, 2},
		{"listed under the line threshold crosses it beyond the slack", []FuncShape{at("tame", lt+1, 70)}, linesMetric, 1, 0, 2},
		{"new long within the slack", []FuncShape{at("fresh", lt, 3)}, linesMetric, 0, 1, 2},
		{"new long beyond the slack", []FuncShape{at("fresh", lt+1, 3)}, linesMetric, 1, 0, 2},
		{"new complex within the slack", []FuncShape{at("fresh", 10, ct)}, complexityMetric, 0, 1, 3},
		{"new complex beyond the slack", []FuncShape{at("fresh", 10, ct+1)}, complexityMetric, 1, 0, 3},
		{"new at the threshold", []FuncShape{at("fresh", LongFuncLines, ComplexFuncThreshold)}, linesMetric, 0, 0, 2},
	} {
		grew, within, shrank := funcShapeCheck(c.shapes, fix, c.m)
		if len(grew) != c.grew || len(within) != c.within || len(shrank) != c.shrank {
			t.Errorf("%s: grew %q, within %q, shrank %q; want %d, %d and %d", c.name, grew, within, shrank,
				c.grew, c.within, c.shrank)
		}
	}
	low := lowerFuncCeilings([]FuncShape{at("big", 390, 90), at("tame", 100, 40), at("fresh", 900, 900)}, fix)
	want := map[string]funcCeiling{"p big": {390, 80}}
	if fmt.Sprint(low) != fmt.Sprint(want) {
		t.Errorf("lowerFuncCeilings = %v, want %v (lower only, drop the gone and the tamed, add nothing)", low, want)
	}
	// Growth within the slack never moves a row up: the lowering keeps the ceiling, not ceiling+slack,
	// and lowers to the measurement once the function shrinks back.
	low = lowerFuncCeilings([]FuncShape{at("big", 400+ls, 80+cs)}, fix)
	if want := map[string]funcCeiling{"p big": {400, 80}}; fmt.Sprint(low) != fmt.Sprint(want) {
		t.Errorf("lowerFuncCeilings over the slack = %v, want %v (the slack must not raise a row)", low, want)
	}
}
