package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// This file pins the CR 702.55a (replicate) and CR 702.66 (squad) arms the
// approx ticket added when it widened Decide's optional-count case from
// "multikick" alone to "multikick", "replicate", "squad". Both asks are the
// same shape as CR 702.43's multikicker count ask -- option 0 is the decline
// (Amount 0) and the options ascend to the largest count the board can still
// pay -- so the arm takes the highest Amount. Each test asserts its own
// precondition (option 0 is the Amount-0 decline and the maximum is 2) so a
// vacuous setup fails loudly instead of passing silently.
//
// The sibling multikick test and the two other pre-existing arms live in
// approx_arms_test.go; these two are split into their own file (2026-09-22
// "new tests go in a new file" rule) so they cannot conflict with a peer
// appending to the shared file.

// TestBotReplicatePaysTheAffordableMaximum is (a): option 0 is "No replicate"
// (Amount 0); the arm pays the largest affordable count instead.
func TestBotReplicatePaysTheAffordableMaximum(t *testing.T) {
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "replicate", Label: "No replicate", Amount: 0},
			{Index: 1, Kind: "replicate", Label: "Pay replicate once", Amount: 1},
			{Index: 2, Kind: "replicate", Label: "Pay replicate 2 times", Amount: 2},
		}}
	// Precondition: option 0 is the decline the default fallback would take,
	// and it differs from the maximum the arm must reach.
	if d.Options[0].Amount != 0 || d.Options[0].Label != "No replicate" {
		t.Fatalf("test setup: option 0 is %+v, want the Amount-0 decline", d.Options[0])
	}
	if d.Options[len(d.Options)-1].Amount != 2 {
		t.Fatalf("test setup: maximum amount is %d, want 2", d.Options[len(d.Options)-1].Amount)
	}
	in := Decide(Board{}, d, rng(1))
	if len(in.Choices) != 1 || in.Choices[0] != 2 {
		t.Fatalf("replicate pick = %v, want [2] (the maximum affordable count)", in.Choices)
	}
}

// TestBotSquadPaysTheAffordableMaximum is (b): option 0 is "No squad"
// (Amount 0); the arm pays the largest affordable count instead.
func TestBotSquadPaysTheAffordableMaximum(t *testing.T) {
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "squad", Label: "No squad", Amount: 0},
			{Index: 1, Kind: "squad", Label: "Pay squad once", Amount: 1},
			{Index: 2, Kind: "squad", Label: "Pay squad 2 times", Amount: 2},
		}}
	// Precondition: option 0 is the decline the default fallback would take,
	// and it differs from the maximum the arm must reach.
	if d.Options[0].Amount != 0 || d.Options[0].Label != "No squad" {
		t.Fatalf("test setup: option 0 is %+v, want the Amount-0 decline", d.Options[0])
	}
	if d.Options[len(d.Options)-1].Amount != 2 {
		t.Fatalf("test setup: maximum amount is %d, want 2", d.Options[len(d.Options)-1].Amount)
	}
	in := Decide(Board{}, d, rng(1))
	if len(in.Choices) != 1 || in.Choices[0] != 2 {
		t.Fatalf("squad pick = %v, want [2] (the maximum affordable count)", in.Choices)
	}
}
