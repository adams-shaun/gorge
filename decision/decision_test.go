package decision

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func sample() *Decision {
	return &Decision{
		Seq: 7, Player: 1, Kind: KPriority, Min: 1, Max: 1,
		Options: []Option{
			{Index: 0, Kind: "cast", Label: "Cast Lightning Bolt", Obj: 3},
			{Index: 1, Kind: "pass", Label: "Pass priority"},
		},
	}
}

func TestValidateRejectsWrongPlayer(t *testing.T) {
	err := sample().Validate(Intent{Seq: 7, Player: 0, Choices: []int{1}})
	if err == nil || !strings.Contains(err.Error(), "player") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsWrongSequence(t *testing.T) {
	err := sample().Validate(Intent{Seq: 6, Player: 1, Choices: []int{1}})
	if err == nil || !strings.Contains(err.Error(), "seq") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsOutOfRangeAndDuplicates(t *testing.T) {
	if err := sample().Validate(Intent{Seq: 7, Player: 1, Choices: []int{2}}); err == nil {
		t.Error("accepted an out-of-range option index")
	}
	if err := sample().Validate(Intent{Seq: 7, Player: 1, Choices: []int{-1}}); err == nil {
		t.Error("accepted a negative option index")
	}
	multi := sample()
	multi.Min, multi.Max = 0, 2
	if err := multi.Validate(Intent{Seq: 7, Player: 1, Choices: []int{1, 1}}); err == nil {
		t.Error("accepted a duplicate choice")
	}
}

func TestValidateEnforcesArity(t *testing.T) {
	d := sample()
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: nil}); err == nil {
		t.Error("accepted too few choices")
	}
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0, 1}}); err == nil {
		t.Error("accepted too many choices")
	}
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0}}); err != nil {
		t.Errorf("rejected a valid intent: %v", err)
	}
}

func TestChosenReturnsOptionsInIntentOrder(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2
	got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1, 0}})
	if len(got) != 2 || got[0].Kind != "pass" || got[1].Kind != "cast" {
		t.Fatalf("Chosen = %+v", got)
	}
}

func TestZeroOptionDecisionAcceptsEmptyIntent(t *testing.T) {
	d := &Decision{Seq: 1, Player: 0, Kind: KBlockers, Min: 0, Max: 0}
	if err := d.Validate(Intent{Seq: 1, Player: 0}); err != nil {
		t.Fatalf("err = %v", err)
	}
	_ = state.PlayerID(0)
}
