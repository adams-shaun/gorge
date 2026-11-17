package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// api:Learn's choice (effects.LearnDecision) lists "nothing" at index 0, then
// "lesson" options, then "rummage". The bot has no arm for it, so Clamp's
// fill must land on index 0, "do nothing", and Validate must accept it.
func TestLearnBotAnswerIsDoNothing(t *testing.T) {
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, Seq: 1,
		ResumeKind: "learn", Options: []decision.Option{
			{Index: 0, Kind: "nothing", Label: "Do nothing"},
			{Index: 1, Kind: "lesson", Label: "Reveal a Lesson", Obj: 5},
			{Index: 2, Kind: "rummage", Label: "Discard a card, then draw a card"},
		}}
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer %+v rejected by Decision.Validate: %v", in, err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("bot learn answer = %v, want the do-nothing option 0", in.Choices)
	}
}
