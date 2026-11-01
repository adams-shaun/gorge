package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// smokeOptions is the smoke workflow: two random-deck games, two sealed
// builds, one 8-seat pod, and two gauntlet heroes against two catalog
// decks — small enough for the test budget, every step exercised.
func smokeOptions(t *testing.T, out string) options {
	return options{
		dir:             testutil.CorpusDirOrSkip(t),
		set:             "FDN",
		seed:            1,
		randomGames:     2,
		player:          "bot@1",
		sealed:          2,
		pods:            1,
		podSeats:        8,
		opponents:       "fdn-limited",
		opponentCount:   2,
		opponentPlayers: "bot,lethal-pressure",
		gauntletHeroes:  2,
		games:           2,
		workers:         4,
		out:             out,
	}
}

// TestLabdemoWorkflowSmoke runs the whole workflow twice into fresh
// directories: every artifact exists and parses (one header per CSV, the
// manifest array in step order), and the second run is byte-identical.
func TestLabdemoWorkflowSmoke(t *testing.T) {
	out1 := t.TempDir()
	var summary1 bytes.Buffer
	if err := run(smokeOptions(t, out1), &summary1); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(summary1.String(), "frozen player bot@1") {
		t.Errorf("summary %q does not name the frozen player", summary1.String())
	}
	if !strings.Contains(summary1.String(), "hero sealed 1 / bot@1") {
		t.Errorf("summary %q does not carry a hero line", summary1.String())
	}

	// draft_data.csv: one header, 3 picks x 14 packs x 8 seats rows, four
	// per-seat draft ids.
	lines := readLines(t, filepath.Join(out1, "draft_data.csv"))
	if len(lines) != 1+3*14*8 {
		t.Fatalf("draft_data.csv has %d lines, want header plus %d picks", len(lines), 3*14*8)
	}
	rows := parseCSV(t, filepath.Join(out1, "draft_data.csv"))
	// The pod's event seed is the workflow's derived pod-0 item, so its
	// per-seat draft ids are "<seed>-s<k>" over that seed.
	podSeed := stepItem(1, saltPods, 0)
	ids := map[string]bool{}
	for _, row := range rows[1:] {
		if row[1] != "gorge_pod8" {
			t.Fatalf("draft_data row event_type %q, want gorge_pod8", row[1])
		}
		ids[row[2]] = true
	}
	if len(ids) != 8 {
		t.Fatalf("%d distinct draft ids, want 8", len(ids))
	}
	for s := 0; s < 8; s++ {
		if !ids[fmt.Sprintf("%d-s%d", podSeed, s)] {
			t.Errorf("draft id %d-s%d missing", podSeed, s)
		}
	}

	// game_data.csv: one header, finished games only, three event types
	// (2 random games, 12 pod games, 2 heroes x 2 opponents x 2 gauntlet
	// games — two rows each).
	grows := parseCSV(t, filepath.Join(out1, "game_data.csv"))
	types := map[string]int{}
	for _, row := range grows[1:] {
		types[row[1]]++
	}
	wantTypes := map[string]int{"gorge_lab": 4, "gorge_pod8": 24, "gorge_gauntlet": 16}
	if len(grows)-1 != 44 {
		t.Fatalf("game_data.csv has %d data rows, want 44", len(grows)-1)
	}
	for typ, n := range wantTypes {
		if types[typ] != n {
			t.Errorf("event type %s has %d rows, want %d", typ, types[typ], n)
		}
	}

	// decks.json: two sealed builds (draft_id "") then the pod's eight
	// decks (per-seat ids).
	var decks []map[string]any
	decodeJSON(t, filepath.Join(out1, "decks.json"), &decks)
	if len(decks) != 10 {
		t.Fatalf("decks.json has %d records, want 10", len(decks))
	}
	for i, want := range []string{"sealed 1", "sealed 2", "p0 FDN draft", "p7 FDN draft"} {
		idx := i
		if i >= 2 {
			idx = 2 + (i - 2) // p0 is deck 2, p7 is deck 9
		}
		if i == 3 {
			idx = 9
		}
		if got := decks[idx]["name"]; got != want {
			t.Errorf("deck %d is %v, want %v", idx, got, want)
		}
	}
	if got := decks[0]["draft_id"]; got != "" {
		t.Errorf("sealed deck 1 draft_id is %v, want the empty string", got)
	}
	if got := decks[1]["draft_id"]; got != "" {
		t.Errorf("sealed deck 2 draft_id is %v, want the empty string", got)
	}
	if got := decks[2]["draft_id"]; got != fmt.Sprintf("%d-s0", podSeed) {
		t.Errorf("pod deck p0 draft_id is %v, want %d-s0", got, podSeed)
	}
	if got := decks[9]["draft_id"]; got != fmt.Sprintf("%d-s7", podSeed) {
		t.Errorf("pod deck p7 draft_id is %v, want %d-s7", got, podSeed)
	}

	// manifests.json: step order — the games manifest, the pod's, then
	// one per gauntlet hero.
	var manifests []map[string]any
	decodeJSON(t, filepath.Join(out1, "manifests.json"), &manifests)
	if len(manifests) != 4 {
		t.Fatalf("manifests.json has %d records, want 4", len(manifests))
	}
	wantKinds := []string{"games", "event", "gauntlet", "gauntlet"}
	for i, want := range wantKinds {
		if got := manifests[i]["kind"]; got != want {
			t.Errorf("manifest %d kind is %v, want %v", i, got, want)
		}
	}

	// The pod's log file exists and carries the pack openings.
	podLog := readLines(t, filepath.Join(out1, "events", "pod1.log.jsonl"))
	if len(podLog) < 8*14 {
		t.Fatalf("pod log has %d entries, want at least the %d pack openings", len(podLog), 8*14)
	}

	// The whole run is a pure function of the flags: a second run over the
	// same flags is byte-identical, every artifact included.
	out2 := t.TempDir()
	var summary2 bytes.Buffer
	if err := run(smokeOptions(t, out2), &summary2); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if stripArtifacts(summary1.String()) != stripArtifacts(summary2.String()) {
		t.Errorf("summaries differ")
	}
	for _, rel := range []string{"draft_data.csv", "game_data.csv", "decks.json", "manifests.json", filepath.Join("events", "pod1.log.jsonl")} {
		a, err := os.ReadFile(filepath.Join(out1, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		b, err := os.ReadFile(filepath.Join(out2, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not byte-identical across runs", rel)
		}
	}
}

// stripArtifacts drops the summary's final "artifacts: <dir>" line: the
// only run-dependent text in it (the two runs write to different temp
// directories).
func stripArtifacts(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-2], "artifacts: ") {
		lines = lines[:len(lines)-2]
	}
	return strings.Join(lines, "\n")
}

// TestLabdemoOptionValidation refuses the out-of-range flags before any
// corpus work.
func TestLabdemoOptionValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*options)
		want string
	}{
		{"random-games", func(o *options) { o.randomGames = -1 }, "-random-games"},
		{"sealed", func(o *options) { o.sealed = -2 }, "-sealed"},
		{"pods", func(o *options) { o.pods = -1 }, "-pods"},
		{"pod-seats", func(o *options) { o.podSeats = 6 }, "-pod-seats"},
		{"opponent-count", func(o *options) { o.opponentCount = -1 }, "-opponent-count"},
		{"gauntlet-heroes", func(o *options) { o.gauntletHeroes = -1 }, "-gauntlet-heroes"},
		{"games", func(o *options) { o.games = 0 }, "-games"},
		{"workers", func(o *options) { o.workers = -1 }, "-workers"},
		{"ext-flush", func(o *options) { o.extFlush = "eager" }, "-ext-flush"},
		{"ext-fail", func(o *options) { o.extFail = "ignore" }, "-ext-fail"},
		{"ext-max-batch", func(o *options) { o.extMaxBatch = -1 }, "-ext-max-batch"},
		{"ext-spec", func(o *options) { o.player = "ext:exec:" }, "bad agent spec"},
	} {
		o := options{seed: 1, podSeats: 8, games: 1}
		tc.mut(&o)
		err := run(o, io.Discard)
		if err == nil {
			t.Errorf("%s: run accepted the bad option", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not name %s", tc.name, err, tc.want)
		}
	}
}

// readLines reads a text artifact and splits it into lines.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// parseCSV reads a CSV artifact into its records.
func parseCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return rows
}

// decodeJSON reads a JSON artifact into out.
func decodeJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
