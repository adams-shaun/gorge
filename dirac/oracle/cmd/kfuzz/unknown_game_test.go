package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestReportNamesUnknownGames: with -implicates-all, a game that recorded an
// option kind with no canonical form prints the pair and seed that posed it, so
// it can be replayed, while still counting and merging as a clean game.
func TestReportNamesUnknownGames(t *testing.T) {
	implicatesAll = true
	t.Cleanup(func() { implicatesAll = false })
	clean := lockstepStub(nil)
	unknown := played{res: oracle.Result{Stats: oracle.Stats{Unknown: map[string]int{
		"priority/ward_waterbend": 2,
	}}}}
	code, stdout := reportOf(t, []played{clean, unknown}, true)
	if code != 1 {
		t.Fatalf("-absolute: exit %d, want 1 for the unknown kind\n%s", code, stdout)
	}
	const want = "unknown-game\t1\t101\tpriority/ward_waterbend\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("no %q in\n%s", want, stdout)
	}
	if !strings.Contains(stdout, ", 2 clean, ") {
		t.Errorf("the unknown kind must still count clean:\n%s", stdout)
	}

	implicatesAll = false
	_, stdout = reportOf(t, []played{unknown}, true)
	if strings.Contains(stdout, "unknown-game") {
		t.Errorf("unknown-game printed without -implicates-all:\n%s", stdout)
	}
}
