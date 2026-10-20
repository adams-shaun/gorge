package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRunPosesStartingPlayerToAgentSeat pins CR 103.1's ask: mtgsim must pose
// the winner-chooses decision to the seats, not silently run the R-9 default
// in Advance before they attach.
func TestRunPosesStartingPlayerToAgentSeat(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not installed")
	}
	agentDir, err := filepath.Abs("../../server/spellbench/seat/testdata/gorgeagent")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	record := filepath.Join(tmp, "choose.jsonl")
	wrapper := filepath.Join(tmp, "agent.sh")
	script := "#!/bin/sh\nexec \"" + goBin + "\" run " + agentDir + "\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPELLBENCH_RECORD", record)
	var buf bytes.Buffer
	if err := runAgents("", 2, 0, 1, true, false, dir, "sb-rec", "", []string{"sb-rec=" + wrapper}, &buf); err != nil {
		t.Fatalf("runAgents: %v\n%s", err, buf.String())
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"kind":"choose_starting_player"`) {
		t.Fatalf("agent seat was never asked to choose the starting player:\n%s", buf.String())
	}
}
