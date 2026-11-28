package rules

// The pad-to-four metamorphic relation (RunOracleScenarioPadded): a two-seat
// scenario replayed with two inert seats after its own must look the same to
// the two original seats, unless a listed rule (seat-sensitive.tsv) makes the
// difference correct.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

const oracleSeatSensitivePath = "testdata/oracle/seat-sensitive.tsv"

var oracleSeatSensitiveClasses = map[string]bool{
	"each-opponent": true, "each-player": true, "player-count": true, "each-turn": true,
	"turn-order": true, "opponent-choice": true, "first-draw": true, "game-end": true, "harness": true,
}

type seatSensitiveRow struct{ class, cr, firstDiff, note string }

// loadSeatSensitive reads seat-sensitive.tsv strictly: its header, sorted
// unique scenarios, a known class, CR ids in the index and a short note.
func loadSeatSensitive(t *testing.T) map[string]seatSensitiveRow {
	t.Helper()
	raw, err := os.ReadFile(oracleSeatSensitivePath)
	if err != nil {
		t.Fatal(err)
	}
	crIDs := map[string]bool{}
	if idx, err := os.ReadFile("crcoverage/cr-index.tsv"); err == nil {
		for _, line := range strings.Split(string(idx), "\n") {
			if id, _, _ := strings.Cut(line, "\t"); id != "" {
				crIDs[id] = true
			}
		}
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != "scenario\tclass\tcr\tfirst_diff\tnote" {
		t.Fatalf("%s: header %q", oracleSeatSensitivePath, lines[0])
	}
	out := map[string]seatSensitiveRow{}
	prev := ""
	for i, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			t.Fatalf("%s:%d: %d columns, want 5", oracleSeatSensitivePath, i+2, len(f))
		}
		if f[0] <= prev {
			t.Errorf("%s:%d: %q is not sorted after %q", oracleSeatSensitivePath, i+2, f[0], prev)
		}
		prev = f[0]
		if !oracleSeatSensitiveClasses[f[1]] {
			t.Errorf("%s:%d: unknown class %q", oracleSeatSensitivePath, i+2, f[1])
		}
		for _, id := range strings.Split(f[2], ",") {
			if id == "" || (len(crIDs) > 0 && !crIDs[id]) {
				t.Errorf("%s:%d: cr %q is not in cr-index.tsv", oracleSeatSensitivePath, i+2, id)
			}
		}
		if f[4] == "" || len(f[4]) > 120 {
			t.Errorf("%s:%d: note must be one sentence of at most 120 characters", oracleSeatSensitivePath, i+2)
		}
		out[f[0]] = seatSensitiveRow{class: f[1], cr: f[2], firstDiff: f[3], note: f[4]}
	}
	return out
}

// TestScenariosHoldAtFourSeats: every in-repo scenario, padded to four seats,
// is the same to its two seats, or differs exactly as its row says.
func TestScenariosHoldAtFourSeats(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rows := loadSeatSensitive(t)
	files := loadOracleFiles(t)
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	type outcome struct{ verdict, diff string }
	got := map[string]*outcome{}
	var keys []string
	for _, p := range paths {
		rel := strings.TrimSuffix(filepath.ToSlash(strings.TrimPrefix(p, filepath.Join("testdata", "oracle")+string(filepath.Separator))), ".json")
		for _, sc := range files[p].Scenarios {
			key := rel + "#" + sc.Name
			keys = append(keys, key)
			got[key] = &outcome{}
		}
	}
	t.Run("scenarios", func(t *testing.T) {
		i := 0
		for _, p := range paths {
			for _, sc := range files[p].Scenarios {
				key, sc := keys[i], sc
				i++
				o := got[key]
				t.Run(key, func(t *testing.T) {
					t.Parallel()
					raw, err := json.Marshal(sc)
					if err != nil {
						t.Fatal(err)
					}
					res, err := RunOracleScenarioPadded(reg, raw, false)
					if err != nil {
						t.Fatal(err)
					}
					o.verdict, o.diff = res.Verdict, res.FirstDiff
					row, listed := rows[key]
					switch {
					case res.Verdict == "differs" && !listed:
						t.Errorf("differs at four seats, no seat-sensitive row: %s\n  pad fails: %v", res.FirstDiff, res.PadFails)
					case res.Verdict == "differs" && row.firstDiff != res.FirstDiff:
						t.Errorf("differs with %q, row says %q", res.FirstDiff, row.firstDiff)
					case res.Verdict != "differs" && listed:
						t.Errorf("verdict %s, but listed as seat-sensitive (%s)", res.Verdict, row.class)
					}
				})
			}
		}
	})
	for k := range rows {
		if got[k] == nil {
			t.Errorf("%s: row names no scenario", k)
		}
	}
	n := map[string]int{}
	for _, k := range keys {
		n[got[k].verdict]++
	}
	t.Logf("same=%d differs=%d base-fails=%d", n["same"], n["differs"], n["base-fails"])
}

func TestPadTurn(t *testing.T) {
	for in, want := range map[int]int{1: 1, 2: 2, 3: 5, 4: 6, 7: 13} {
		if got := padTurn(in); got != want {
			t.Errorf("padTurn(%d) = %d, want %d", in, got, want)
		}
	}
}

const padShock = `{"name":"shock","setup":{"p0":{"hand":["Shock"]},"p1":{}},` +
	`"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":0}],` +
	`"expect":[{"life":{"p1":18}}]}`

func runPadded(t *testing.T, raw string) OraclePadResult {
	t.Helper()
	res, err := RunOracleScenarioPadded(testutil.CorpusRegistry(t), []byte(raw), false)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPaddedShockIsSame(t *testing.T) {
	res := runPadded(t, padShock)
	if res.Verdict != "same" {
		t.Fatalf("verdict %s (%s), base fails %v, pad fails %v", res.Verdict, res.FirstDiff, res.BaseFails, res.PadFails)
	}
}

func TestPaddedEachOpponentDiffers(t *testing.T) {
	rift := runPadded(t, `{"name":"rift","setup":{"p0":{"hand":["Flame Rift"]},"p1":{}},`+
		`"steps":[{"op":"cast","seat":0,"card":"p0:Flame Rift","mana":"RR"},{"op":"resolve","seat":0}],`+
		`"expect":[{"life":{"p0":16,"p1":16}}]}`)
	if rift.Verdict != "same" {
		t.Fatalf("Flame Rift: verdict %s (%s), base fails %v, pad fails %v", rift.Verdict, rift.FirstDiff, rift.BaseFails, rift.PadFails)
	}
	syphon := runPadded(t, `{"name":"syphon","setup":{"p0":{"hand":["Syphon Soul"]},"p1":{}},`+
		`"steps":[{"op":"cast","seat":0,"card":"p0:Syphon Soul","mana":"BBB"},{"op":"resolve","seat":0}],`+
		`"expect":[{"life":{"p0":22,"p1":18}}]}`)
	if len(syphon.BaseFails) != 0 {
		t.Fatalf("Syphon Soul two-seat run fails: %v", syphon.BaseFails)
	}
	if syphon.Verdict != "differs" || syphon.FirstDiff != "snap 2 p0.life: 22 != 26" {
		t.Fatalf("Syphon Soul: verdict %s, first diff %q; want differs at snap 2 p0.life: 22 != 26", syphon.Verdict, syphon.FirstDiff)
	}
}

// TestPaddedPassToSkipsInertTurns: from p1's turn, pass_to draw stops in
// p0's next draw step, the inert p2 and p3 turns in between skipped. The draw
// step (not upkeep) is the one that tells: were pass_to to stop in p2's draw,
// the step loop's skipInert would drive on to p0's upkeep, not its draw.
//
// The decision-keyed gate (!r.inert(d.Player)) has no test: an inert seat
// only holds a non-priority decision in a pass_to loop when it has a real
// choice to make (a cleanup discard after eight inert draws, an opponent's
// "each player chooses"), and no scenario here reaches one.
func TestPaddedPassToSkipsInertTurns(t *testing.T) {
	raw := `{"name":"pass","turn":2,"setup":{"p0":{},"p1":{}},"steps":[{"op":"pass_to","step":"draw"}]}`
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	pad, ok := padOracleScenario(sc)
	if !ok {
		t.Fatal("two-seat scenario refused")
	}
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"base", "pad"} {
		s := map[string]oracleScenario{"base": sc, "pad": pad}[name]
		fails, log, run := runOracleScenarioSpare(reg, s, false, nil)
		if len(fails) != 0 {
			t.Fatalf("%s: %v\n%s", name, fails, strings.Join(log, "\n"))
		}
		first, last := run.snaps[0], run.snaps[len(run.snaps)-1]
		if first.Active != 1 {
			t.Fatalf("%s: setup active p%d, want p1", name, first.Active)
		}
		if last.Active != 0 || last.Step != "draw" {
			t.Errorf("%s: stopped in p%d's %s, want p0's draw", name, last.Active, last.Step)
		}
		if want := map[string]int32{"base": 3, "pad": 5}[name]; last.Turn != want {
			t.Errorf("%s: stopped on turn %d, want %d", name, last.Turn, want)
		}
	}
	if res := runPadded(t, raw); res.Verdict != "same" {
		t.Errorf("verdict %s (%s)", res.Verdict, res.FirstDiff)
	}
}

func TestPaddedTwoSeatRunUnchanged(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func() []byte {
		res, err := RunOracleScenarioJSON(reg, []byte(padShock))
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	before := run()
	if res := runPadded(t, padShock); res.Verdict != "same" {
		t.Fatalf("padded verdict %s (%s)", res.Verdict, res.FirstDiff)
	}
	if after := run(); !reflect.DeepEqual(before, after) {
		t.Errorf("two-seat result changed after a padded run:\n%s\n%s", before, after)
	}
}

func TestPaddedRefusesThreeSeats(t *testing.T) {
	res := runPadded(t, `{"name":"three","players":3,"setup":{"p0":{},"p1":{},"p2":{}},"steps":[]}`)
	if res.Verdict != "not-two-seat" {
		t.Errorf("verdict %s, want not-two-seat", res.Verdict)
	}
	// An invalid players count is the harness error, not "not-two-seat".
	bad := `{"name":"five","players":5,"setup":{"p0":{},"p1":{}},"steps":[]}`
	if res, err := RunOracleScenarioPadded(testutil.CorpusRegistry(t), []byte(bad), false); err == nil {
		t.Errorf("players 5: verdict %s, want an error", res.Verdict)
	}
}
