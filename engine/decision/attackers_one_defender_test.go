package decision

import (
	"strings"
	"testing"
)

// TestValidateRejectsOneAttackerAgainstTwoDefenders pins CR 508.1 at the wire
// fence: the KAttackers option list publishes one option per (attacker,
// defender) pair, so a client can select the same creature for two defenders.
// The engine's validateAttackers refuses that declaration, so Validate must
// refuse it first and let the seat re-answer instead of crashing the match on
// Submit (D1a).
func TestValidateRejectsOneAttackerAgainstTwoDefenders(t *testing.T) {
	d := Decision{
		Seq: 7, Player: 1, Kind: KAttackers, Min: 0, Max: 3,
		Options: []Option{
			{Index: 0, Obj: 38, Label: "attacker 38 -> player 2"},
			{Index: 1, Obj: 38, Label: "attacker 38 -> player 3"},
			{Index: 2, Obj: 39, Label: "attacker 39 -> player 2"},
		},
	}
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0, 1}}); err == nil {
		t.Fatal("one attacker declared against two defenders was accepted")
	} else if !strings.Contains(err.Error(), "attacker 38 declared against more than one defender") {
		t.Fatalf("error = %q, want the engine's multi-defender reason", err)
	}
	// The legal shapes must still pass: one defender per attacker, and either
	// creature alone.
	for _, choices := range [][]int{{0}, {2}, {0, 2}} {
		if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: choices}); err != nil {
			t.Fatalf("choices %v rejected: %v", choices, err)
		}
	}
	// Two different defenders are fine when they name different attackers.
	d.Options[1].Obj = 39
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0, 1}}); err != nil {
		t.Fatalf("distinct attackers rejected: %v", err)
	}
}
