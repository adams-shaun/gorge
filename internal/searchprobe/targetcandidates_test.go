package searchprobe

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

func targetKeys(t *testing.T, d *decision.Decision, got []decision.Intent) string {
	t.Helper()
	var keys []int
	for _, in := range got {
		if err := d.Validate(in); err != nil {
			t.Fatalf("invalid candidate %v: %v", in.Choices, err)
		}
		if len(in.Choices) != 1 {
			t.Fatalf("candidate %v is not one choice", in.Choices)
		}
		keys = append(keys, in.Choices[0])
	}
	return fmt.Sprint(keys)
}

// A player-or-permanent target (a burn spell's "any target"): the bot's pick
// first, then every other option in index order, player options included.
func TestTargetCandidatesPlayersAndPermanents(t *testing.T) {
	d := &decision.Decision{Kind: decision.KTarget, Seq: 4, Player: 0, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "target", Player: 0, Label: "you"},
		{Index: 1, Kind: "target", Player: 1, Label: "opponent"},
		{Index: 2, Kind: "target", Obj: 20},
		{Index: 3, Kind: "target", Obj: 21},
	}}
	bot := decision.Intent{Seq: 4, Player: 0, Choices: []int{2}}
	if got := targetKeys(t, d, TargetCandidates(d, bot, 8)); got != "[2 0 1 3]" {
		t.Fatalf("candidates %s, want [2 0 1 3]", got)
	}
	if got := targetKeys(t, d, TargetCandidates(d, bot, 2)); got != "[2 0]" {
		t.Fatalf("limit 2 candidates %s, want [2 0]", got)
	}
	if TargetCandidates(d, bot, 1) != nil {
		t.Fatal("limit 1 leaves nothing to compare; want nil")
	}
}

// Every shape outside the single-choice, unbudgeted KTarget is the bot's.
func TestTargetCandidatesRefusals(t *testing.T) {
	opts := []decision.Option{{Index: 0, Kind: "target", Obj: 20}, {Index: 1, Kind: "target", Obj: 21}, {Index: 2, Kind: "target", Obj: 22}}
	bot := decision.Intent{Seq: 1, Player: 0, Choices: []int{0}}
	for name, d := range map[string]*decision.Decision{
		"max 2":      {Kind: decision.KTarget, Seq: 1, Min: 1, Max: 2, Options: opts},
		"min 0":      {Kind: decision.KTarget, Seq: 1, Min: 0, Max: 1, Options: opts},
		"max sum":    {Kind: decision.KTarget, Seq: 1, Min: 1, Max: 1, MaxSum: 4, Options: opts},
		"budgeted":   {Kind: decision.KTarget, Seq: 1, Min: 1, Max: 1, Budgeted: true, Options: opts},
		"one option": {Kind: decision.KTarget, Seq: 1, Min: 1, Max: 1, Options: opts[:1]},
		"not target": {Kind: decision.KChoose, Seq: 1, Min: 1, Max: 1, Options: opts},
	} {
		if got := TargetCandidates(d, bot, 8); got != nil {
			t.Errorf("%s: got %d candidates, want nil", name, len(got))
		}
	}
	d := &decision.Decision{Kind: decision.KTarget, Seq: 1, Min: 1, Max: 1, Options: opts}
	if !SingleTarget(d) {
		t.Fatal("the single-target shape must be admitted")
	}
	if TargetCandidates(d, decision.Intent{Seq: 1, Choices: []int{0, 1}}, 8) != nil {
		t.Error("a two-choice bot answer has no baseline to beat; want nil")
	}
	if TargetCandidates(d, decision.Intent{Seq: 1, Choices: []int{7}}, 8) != nil {
		t.Error("an invalid bot answer has no baseline to beat; want nil")
	}
}
