package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// A per-defender attack ceiling (AttackRestrict's MaxAttackers$ scoped by
// ValidDefender$) is Decision.GroupLimits: each defended player is its own
// Group and two scoped restrictions can cap different defenders differently,
// which the decision-wide scalar GroupLimit cannot express. GroupCapFor is
// the one reader Validate, FitRequired and Clamp share; these tests pin that
// differing caps are honoured per group through the bot's repair, so a bot
// answer can never be one Validate rejects (the livelock class).

// TestBotClampHonoursPerGroupLimits repairs an answer that over-selects one
// group while staying within another group's higher cap: the "low" group is
// capped at one, the "high" group at three. Clamp must trim the low group to
// its own cap and keep the high group's picks -- a single decision-wide cap
// would either admit the low over-pick or needlessly trim the high group.
func TestBotClampHonoursPerGroupLimits(t *testing.T) {
	d := decision.Decision{
		Kind: decision.KAttackers,
		Min:  0,
		Max:  5,
		GroupLimits: map[string]int{
			"def:0": 1,
			"def:2": 3,
		},
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 1, Player: 0, Group: "def:0"},
			{Index: 1, Kind: "attacker", Obj: 2, Player: 0, Group: "def:0"},
			{Index: 2, Kind: "attacker", Obj: 3, Player: 2, Group: "def:2"},
			{Index: 3, Kind: "attacker", Obj: 4, Player: 2, Group: "def:2"},
			{Index: 4, Kind: "attacker", Obj: 5, Player: 2, Group: "def:2"},
		},
	}
	if d.GroupCapFor("def:0") != 1 || d.GroupCapFor("def:2") != 3 {
		t.Fatalf("precondition: GroupCapFor = (%d,%d), want (1,3)", d.GroupCapFor("def:0"), d.GroupCapFor("def:2"))
	}
	over := decision.Intent{Choices: []int{0, 1, 2, 3, 4}}
	if err := d.Validate(over); err == nil {
		t.Fatal("precondition: the over-cap raw answer must be rejected by Validate")
	}
	in := Clamp(&d, over)
	if err := d.Validate(in); err != nil {
		t.Fatalf("Clamp left an answer Validate rejects: choices %v, err %v", in.Choices, err)
	}
	low, high := 0, 0
	for _, c := range in.Choices {
		switch d.Options[c].Group {
		case "def:0":
			low++
		case "def:2":
			high++
		}
	}
	if low != 1 {
		t.Fatalf("repaired low-cap group kept %d picks, want 1 (choices %v)", low, in.Choices)
	}
	if high != 3 {
		t.Fatalf("repaired high-cap group kept %d picks, want 3 (choices %v)", high, in.Choices)
	}
}

// TestGroupCapForFallsBackToDecisionWideCap pins the fallback: an absent
// group and a group mapped below 2 read the decision-wide GroupCap, so every
// existing decision (GroupLimit 0 or set by EACH) is unchanged.
func TestGroupCapForFallsBackToDecisionWideCap(t *testing.T) {
	d := decision.Decision{GroupLimit: 2, GroupLimits: map[string]int{"a": 1, "b": 4}}
	if got := d.GroupCapFor("a"); got != 2 {
		t.Fatalf("GroupCapFor(a) = %d, want the decision-wide cap 2", got)
	}
	if got := d.GroupCapFor("b"); got != 4 {
		t.Fatalf("GroupCapFor(b) = %d, want its own raised cap 4", got)
	}
	if got := d.GroupCapFor("absent"); got != 2 {
		t.Fatalf("GroupCapFor(absent) = %d, want the decision-wide cap 2", got)
	}
}
