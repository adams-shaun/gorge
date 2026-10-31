package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// extOptions is the external-agent workflow: every role (frozen player,
// drafter, builder) names one ext:exec: spec — this test binary
// re-executed as a refagent child serving the first policy with every
// view_omit name — over a small FDN workflow, the pod's tapes verified.
func extAgentOptions(t *testing.T, out string, workers int, player string) options {
	t.Helper()
	names := make([]string, 0, len(wire.ViewOmitNames))
	for _, e := range wire.ViewOmitNames {
		names = append(names, e.Name)
	}
	helper, err := json.Marshal(helperConfig{Policy: "first", ModelVersion: "1", ViewOmit: names})
	if err != nil {
		t.Fatalf("marshal helper config: %v", err)
	}
	t.Setenv("GORGE_EXTAGENT_HELPER", string(helper))
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs %s: %v", os.Args[0], err)
	}
	spec := "ext:exec:" + self
	if player == "" {
		player = spec
	}
	return options{
		dir:             testutil.CorpusDirOrSkip(t),
		set:             "FDN",
		seed:            1,
		randomGames:     2,
		player:          player,
		sealed:          2,
		pods:            1,
		podSeats:        4,
		opponents:       "fdn-limited",
		opponentCount:   2,
		opponentPlayers: "bot,lethal-pressure",
		gauntletHeroes:  1,
		games:           1,
		workers:         workers,
		drafter:         spec,
		builder:         spec,
		verifyTapes:     true,
		out:             out,
	}
}

// TestLabdemoExternalAgent drives the whole workflow with one external
// agent in every agent role: run once at workers 1 and once at workers 4
// into separate directories — both succeed, both stdouts are equal, and
// the two directories hold the same file names with identical bytes
// (batching may vary with scheduling; results and artifacts may not).
func TestLabdemoExternalAgent(t *testing.T) {
	runOne := func(workers int) (stdout, errOut string, files map[string][]byte) {
		out := t.TempDir()
		var so, eo bytes.Buffer
		o := extAgentOptions(t, out, workers, "")
		o.errOut = &eo
		if err := run(o, &so); err != nil {
			t.Fatalf("run (workers %d): %v", workers, err)
		}
		return so.String(), eo.String(), readArtifactTree(t, out)
	}
	stdout1, errOut1, files1 := runOne(1)
	stdout4, errOut4, files4 := runOne(4)
	if stripArtifacts(stdout1) != stripArtifacts(stdout4) {
		t.Errorf("stdouts differ:\n--- workers 1\n%s\n--- workers 4\n%s", stdout1, stdout4)
	}
	if errOut4 == "" || !strings.Contains(errOut4, "extern ext:first@1: picks ") {
		t.Errorf("workers 4 errOut %q does not carry the agent's stats line", errOut4)
	}
	if len(files1) == 0 {
		t.Fatal("the run wrote no artifacts")
	}
	if len(files1) != len(files4) {
		t.Fatalf("file sets differ: %d vs %d names", len(files1), len(files4))
	}
	for name, data1 := range files1 {
		data4, ok := files4[name]
		if !ok {
			t.Errorf("workers 4 lacks artifact %s", name)
			continue
		}
		if !bytes.Equal(data1, data4) {
			t.Errorf("artifact %s is not byte-identical across the two runs", name)
		}
	}

	// The manifests name the one agent under all three roles (its ref is
	// ext:<hello agent>@<model version>, the first policy's ref).
	manifests := string(files1["manifests.json"])
	for _, want := range []string{"player:ext:first@1", "drafter:ext:first@1", "builder:ext:first@1"} {
		if !strings.Contains(manifests, want) {
			t.Errorf("manifests.json lacks %s", want)
		}
	}

	// errOut holds exactly one extern stats line: one agent dialed once
	// served all three roles.
	n := 0
	for _, line := range strings.Split(strings.TrimSuffix(errOut1, "\n"), "\n") {
		if strings.HasPrefix(line, "extern ext:first@1: picks ") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("errOut holds %d extern stats lines, want 1:\n%s", n, errOut1)
	}
	if !strings.Contains(errOut1, "extern ext:first@1: picks ") {
		t.Errorf("errOut %q does not carry the agent's stats line", errOut1)
	}
}

// TestLabdemoExternalVersionPin: a spec that pins model_version 2 against
// a helper serving model_version 1 is a draftlab.ErrVersion freeze error,
// the same rule the built-ins follow.
func TestLabdemoExternalVersionPin(t *testing.T) {
	o := extAgentOptions(t, t.TempDir(), 1, "")
	o.player += "@2"
	err := run(o, io.Discard)
	if err == nil {
		t.Fatal("run accepted a pinned version the agent does not serve")
	}
	if !errors.Is(err, draftlab.ErrVersion) {
		t.Errorf("error %v does not wrap draftlab.ErrVersion", err)
	}
}

// TestLabdemoDefaultFlagsUnchanged: the smoke workflow run with the new
// fields at their zero values and with them at their explicit defaults
// produces identical stdout and artifacts — the registry resolves the
// defaults to exactly the built-ins the workflow hard-coded before E5d-5.
func TestLabdemoDefaultFlagsUnchanged(t *testing.T) {
	out1, out2 := t.TempDir(), t.TempDir()
	var so1, so2 bytes.Buffer
	if err := run(smokeOptions(t, out1), &so1); err != nil {
		t.Fatalf("zero-value run: %v", err)
	}
	o2 := smokeOptions(t, out2)
	o2.drafter, o2.builder = "rating-colour@1", "best23@1"
	if err := run(o2, &so2); err != nil {
		t.Fatalf("default-flag run: %v", err)
	}
	if stripArtifacts(so1.String()) != stripArtifacts(so2.String()) {
		t.Errorf("summaries differ")
	}
	files1 := readArtifactTree(t, out1)
	files2 := readArtifactTree(t, out2)
	if len(files1) != len(files2) {
		t.Fatalf("file sets differ: %d vs %d names", len(files1), len(files2))
	}
	for name, data1 := range files1 {
		if !bytes.Equal(data1, files2[name]) {
			t.Errorf("artifact %s is not byte-identical between the zero-value and default-flag runs", name)
		}
	}
}

// readArtifactTree reads every file under dir into a map keyed by its
// slash-separated path relative to dir.
func readArtifactTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}
