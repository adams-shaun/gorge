package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// shapeRows runs o with Shape pointed at a fresh temporary file, then
// decodes every JSON line of that file. It returns the decoded rows and the
// run's stdout.
func shapeRows(t *testing.T, o simOptions) ([]map[string]any, string) {
	t.Helper()
	o.Shape = filepath.Join(t.TempDir(), "shape.jsonl")
	var out bytes.Buffer
	if err := runSim(o, &out); err != nil {
		t.Fatalf("runSim: %v\n%s", err, out.String())
	}
	return readShapeRows(t, o.Shape), out.String()
}

func readShapeRows(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading shape file: %v", err)
	}
	var rows []map[string]any
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("shape line %d is not JSON: %v: %q", i+1, err, line)
		}
		rows = append(rows, row)
	}
	return rows
}

func jsonFloat(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	f, ok := m[key].(float64)
	if !ok {
		t.Fatalf("field %q = %#v, want a number", key, m[key])
	}
	return f
}

func jsonInt(t *testing.T, m map[string]any, key string) int {
	t.Helper()
	return int(jsonFloat(t, m, key))
}

// checkOptionStats asserts every option stat's max >= p95 >= 0 and that its
// histogram sums to its n, and returns the sum of the n counts.
func checkOptionStats(t *testing.T, options map[string]any) int {
	t.Helper()
	nSum := 0
	for kind, raw := range options {
		stat, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s: stat = %#v", kind, raw)
		}
		n := jsonInt(t, stat, "n")
		p95 := jsonFloat(t, stat, "p95")
		max := jsonFloat(t, stat, "max")
		if p95 < 0 || max < p95 {
			t.Errorf("%s: max %v, p95 %v, want max >= p95 >= 0", kind, max, p95)
		}
		hist, ok := stat["hist"].(map[string]any)
		if !ok {
			t.Fatalf("%s: hist = %#v, want an object", kind, stat["hist"])
		}
		hsum := 0
		for count, rawN := range hist {
			if _, err := strconv.Atoi(count); err != nil {
				t.Errorf("%s: histogram key %q is not a decimal count", kind, count)
			}
			f, ok := rawN.(float64)
			if !ok {
				t.Fatalf("%s: histogram entry %q = %#v", kind, count, rawN)
			}
			hsum += int(f)
		}
		if hsum != n {
			t.Errorf("%s: histogram sums to %d, n is %d", kind, hsum, n)
		}
		nSum += n
	}
	return nSum
}

// TestShapeLeavesTheGameAlone: -shape only observes, so a verified 2-seat
// constructed game prints exactly the same bytes with and without it,
// including the chain head and the replay confirmation.
func TestShapeLeavesTheGameAlone(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	base := simOptions{Seats: 2, Games: 1, Seed: 0, Verify: true, Dir: dir, Format: "constructed"}

	var plain bytes.Buffer
	if err := runSim(base, &plain); err != nil {
		t.Fatalf("run without -shape: %v\n%s", err, plain.String())
	}
	shaped := base
	shaped.Shape = filepath.Join(t.TempDir(), "shape.jsonl")
	var with bytes.Buffer
	if err := runSim(shaped, &with); err != nil {
		t.Fatalf("run with -shape: %v\n%s", err, with.String())
	}

	if plain.String() != with.String() {
		t.Errorf("-shape changed stdout:\nwithout:\n%s\nwith:\n%s", plain.String(), with.String())
	}
	for _, want := range []string{"chain=", "replay OK"} {
		if !strings.Contains(with.String(), want) {
			t.Errorf("stdout missing %q: %q", want, with.String())
		}
	}
	if _, err := os.Stat(shaped.Shape); err != nil {
		t.Errorf("shape file was not written: %v", err)
	}
}

// TestShapeRowsMatchTheSummary: two constructed games write two game rows
// and one aggregate row; each game row's decisions and turns equal the
// numbers on its stdout line, and the option statistics are internally
// consistent with the decisions they summarize.
func TestShapeRowsMatchTheSummary(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	rows, out := shapeRows(t, simOptions{Seats: 2, Games: 2, Dir: dir, Format: "constructed"})
	if len(rows) != 3 {
		t.Fatalf("wrote %d lines, want 3 (2 games + 1 aggregate)", len(rows))
	}

	re := regexp.MustCompile(`(\d+) intents,.*?(\d+) turns`)
	summary := re.FindAllStringSubmatch(out, -1)
	if len(summary) != 2 {
		t.Fatalf("stdout has %d game summary lines, want 2:\n%s", len(summary), out)
	}

	totalDecisions := 0
	for i := 0; i < 2; i++ {
		g := rows[i]
		if g["row"] != "game" {
			t.Fatalf("line %d row = %v, want game", i+1, g["row"])
		}
		wantDecisions, err := strconv.Atoi(summary[i][1])
		if err != nil {
			t.Fatal(err)
		}
		wantTurns, err := strconv.Atoi(summary[i][2])
		if err != nil {
			t.Fatal(err)
		}
		if got := jsonInt(t, g, "decisions"); got != wantDecisions {
			t.Errorf("game %d: decisions = %d, stdout says %d", i+1, got, wantDecisions)
		}
		if got := jsonInt(t, g, "turns"); got != wantTurns {
			t.Errorf("game %d: turns = %d, stdout says %d", i+1, got, wantTurns)
		}
		options, ok := g["options"].(map[string]any)
		if !ok {
			t.Fatalf("game %d: options = %#v, want an object", i+1, g["options"])
		}
		nSum := checkOptionStats(t, options)
		if nSum != wantDecisions {
			t.Errorf("game %d: option n's sum to %d, decisions is %d", i+1, nSum, wantDecisions)
		}
		totalDecisions += wantDecisions
	}

	agg := rows[2]
	if agg["row"] != "aggregate" {
		t.Fatalf("last row = %v, want aggregate", agg["row"])
	}
	if got := jsonInt(t, agg, "games"); got != 2 {
		t.Errorf("aggregate games = %d, want 2", got)
	}
	options, ok := agg["options"].(map[string]any)
	if !ok {
		t.Fatalf("aggregate options = %#v, want an object", agg["options"])
	}
	if got := checkOptionStats(t, options); got != totalDecisions {
		t.Errorf("aggregate option n's sum to %d, the games' decisions sum to %d", got, totalDecisions)
	}
}

// TestShapeIsDeterministic: two runs with the same flags write byte-identical
// shape files.
func TestShapeIsDeterministic(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	run := func(path string) []byte {
		t.Helper()
		o := simOptions{Seats: 2, Games: 2, Dir: dir, Format: "constructed", Shape: path}
		var out bytes.Buffer
		if err := runSim(o, &out); err != nil {
			t.Fatalf("runSim: %v\n%s", err, out.String())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading shape file: %v", err)
		}
		return data
	}
	a := run(filepath.Join(t.TempDir(), "a.jsonl"))
	b := run(filepath.Join(t.TempDir(), "b.jsonl"))
	if !bytes.Equal(a, b) {
		t.Errorf("shape files differ:\n%s\n---\n%s", a, b)
	}
}

// TestShapeCountsCommanderCasts: a commander run reports its format, casts
// a commander and reaches the wide arena a Commander State must hold. Seed
// 0 is pinned because it is the first seed that satisfies both: it casts a
// commander twice and peaks at 205 arena objects.
func TestShapeCountsCommanderCasts(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	rows, _ := shapeRows(t, simOptions{
		Decks:  strings.Join(commanderPair, ","),
		Seats:  2,
		Games:  1,
		Seed:   0,
		Dir:    dir,
		Format: "commander",
	})
	if len(rows) != 2 {
		t.Fatalf("wrote %d lines, want 2 (1 game + 1 aggregate)", len(rows))
	}
	g := rows[0]
	if got := g["kind"]; got != "commander/2" {
		t.Errorf("kind = %v, want commander/2", got)
	}
	if got := jsonInt(t, g, "commander_casts"); got < 1 {
		t.Errorf("commander_casts = %d, want >= 1", got)
	}
	peak, ok := g["peak"].(map[string]any)
	if !ok {
		t.Fatalf("peak = %#v, want an object", g["peak"])
	}
	if got := jsonInt(t, peak, "objects"); got < 200 {
		t.Errorf("peak.objects = %d, want >= 200", got)
	}
}
