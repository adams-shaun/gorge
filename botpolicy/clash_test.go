package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

func TestClashPlacementBotAnswerValidates(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 1, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt:  "Put the revealed card on top or bottom of your library",
		Options: []decision.Option{{Index: 0, Kind: "bottom", Label: "Put it on the bottom", Player: 1}, {Index: 1, Kind: "top", Label: "Keep it on top", Player: 1}}}
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer %+v rejected by Decision.Validate: %v", in, err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("no-host-safe bot answer = %v, want bottom option 0", in.Choices)
	}
}
