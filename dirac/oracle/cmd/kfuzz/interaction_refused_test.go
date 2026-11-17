package main

import (
	"strings"
	"testing"
)

// -play-interaction-refused keeps the guard's reason on the record: a game
// played inside a refused pair is counted as such, and a clean one says the
// guard's prediction was too wide.
func TestPlayInteractionRefusedAccounting(t *testing.T) {
	clean := lockstepStub(nil)
	replayed := clean
	replayed.refusedPair = "chooseplayerorder"
	code, stdout := reportOf(t, []played{clean, replayed}, true)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (a clean replay is not a failure)\n%s", code, stdout)
	}
	for _, want := range []string{
		"fdn: 2 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent",
		"interaction-refused: played 1 (1 clean), chooseplayerorder=1",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in\n%s", want, stdout)
		}
	}
}
