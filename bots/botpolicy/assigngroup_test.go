package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBotAssignGroupMakesSelfAFriendAndOthersFoes pins the bot arm for
// api:AssignGroup's per-player ask: the bot's own seat goes to the first group
// (Friend), every other seat to the last (Foe), and each answer validates.
func TestBotAssignGroupMakesSelfAFriendAndOthersFoes(t *testing.T) {
	for _, tc := range []struct {
		about state.PlayerID
		want  int
	}{{0, 0}, {1, 1}, {2, 1}} {
		d := &decision.Decision{Seq: 3, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
			ResumeKind: "assigngroup", Options: []decision.Option{
				{Index: 0, Kind: "group", Label: "Friend", Player: tc.about},
				{Index: 1, Kind: "group", Label: "Foe", Player: tc.about},
			}}
		in := Decide(Board{}, d, rng(1))
		if err := d.Validate(in); err != nil {
			t.Fatalf("about seat %d: bot answer fails Validate: %v", tc.about, err)
		}
		if len(in.Choices) != 1 || in.Choices[0] != tc.want {
			t.Errorf("about seat %d: bot chose %v, want [%d]", tc.about, in.Choices, tc.want)
		}
	}
}
