package kgen

import (
	"strings"
	"testing"
)

// TestOutOfTheTombsDrawReplBody pins the one graveyard movement a
// DrawProgram body may carry: Out of the Tombs' mandatory hidden pick of a
// creature card you own from your graveyard to the battlefield (Remembering
// it), the Remembered-gated LosesGame branch, and the Cleanup that clears the
// memory. The lowered dump names the movement and the loss so a drift back to
// the broad refusal, or to a different movement, fails here.
func TestOutOfTheTombsDrawReplBody(t *testing.T) {
	c := lowerNamedCard(t, "Out of the Tombs")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals = %v, want none", c.Refusals)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	dump := sb.String()
	for _, want := range []string{
		"repl DrawProgram",
		"ChooseMove obj=You A=1 C=1 CS=from=Graveyard|Remember|Mandatory F={types=0x2 owner=1}",
		"SkipIf if=RefMatches({types=0x0},Remembered,op3,0) skip=1",
		"LoseGame obj=You",
		"ClearRemembered",
	} {
		if !strings.Contains(dump, want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, dump)
		}
	}
}
