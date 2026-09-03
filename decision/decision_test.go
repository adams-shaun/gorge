package decision

import (
	"encoding/json"
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

// TestOptionPlayer0AlwaysEmitted verifies that Player field is emitted even when
// the value is 0, since 0 is a valid seat. Obj field still has omitempty because
// ObjID 0 means "no object".
func TestOptionPlayer0AlwaysEmitted(t *testing.T) {
	opt := Option{
		Index:  0,
		Kind:   "test",
		Label:  "Test",
		Player: 0, // Player 0 is valid
		Obj:    0, // Obj 0 means "no object"
	}
	data, err := json.Marshal(opt)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	jsonStr := string(data)

	// The player field should be present
	if !strings.Contains(jsonStr, `"player":0`) {
		t.Errorf("JSON missing 'player':0, got: %s", jsonStr)
	}

	// The obj field should NOT be present (omitempty applies)
	if strings.Contains(jsonStr, `"obj"`) {
		t.Errorf("JSON should not contain 'obj' field when 0, got: %s", jsonStr)
	}
}

// TestChosenBoundsChecking verifies that Chosen returns nil for out-of-range indices
// and does not panic, and follows the all-or-nothing rule.
func TestChosenBoundsChecking(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2

	// Test index equal to len(Options)
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{2}}); got != nil {
		t.Errorf("Expected nil for index 2 (len=%d), got %+v", len(d.Options), got)
	}

	// Test negative index
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{-1}}); got != nil {
		t.Error("Expected nil for negative index, got non-nil")
	}

	// Test large index
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1000}}); got != nil {
		t.Error("Expected nil for large index, got non-nil")
	}

	// Test mixed valid and invalid indices (all-or-nothing: should return nil)
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{0, 2}}); got != nil {
		t.Error("Expected nil for mixed valid/invalid indices, got non-nil")
	}
}

// TestChosenHappyPath verifies that valid intents still return the expected options
// in order, so the bounds guard did not break the normal case.
func TestChosenHappyPath(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2

	// Test with valid choices
	got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1, 0}})
	if got == nil {
		t.Fatal("Expected non-nil for valid intent, got nil")
	}
	if len(got) != 2 {
		t.Fatalf("Expected 2 options, got %d", len(got))
	}
	if got[0].Kind != "pass" || got[0].Index != 1 {
		t.Errorf("First option should be pass (index 1), got %+v", got[0])
	}
	if got[1].Kind != "cast" || got[1].Index != 0 {
		t.Errorf("Second option should be cast (index 0), got %+v", got[1])
	}

	// Test with single valid choice
	got = d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{0}})
	if got == nil {
		t.Fatal("Expected non-nil for valid single choice, got nil")
	}
	if len(got) != 1 || got[0].Kind != "cast" {
		t.Fatalf("Expected single cast option, got %+v", got)
	}

	// Test with empty choices (still valid if Min is 0)
	got = d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{}})
	if got == nil {
		t.Fatal("Expected non-nil for empty valid choices, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("Expected empty result, got %d options", len(got))
	}
}
