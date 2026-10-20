package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestRunPlaysASpellbenchAgent(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	script, err := filepath.Abs("../../server/spellbench/seat/testdata/minimal_bot.py")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	spec := "sb-minimal=" + script
	if err := runAgents("", 2, 0, 1, true, false, dir, "sb-minimal", "", []string{spec}, &buf); err != nil {
		t.Fatalf("runAgents: %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "replay OK") {
		t.Fatalf("agent game did not replay:\n%s", out)
	}
	if !strings.Contains(out, "winner=") && !strings.Contains(out, "draw") {
		t.Fatalf("agent game produced no result:\n%s", out)
	}
}
