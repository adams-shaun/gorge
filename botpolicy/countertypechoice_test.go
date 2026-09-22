package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestCounterKindChoiceBotAnswerValidates keeps the deterministic no-host and
// bot convention honest: the bot takes the first two distinct counter kinds.
func TestCounterKindChoiceBotAnswerValidates(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 2, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "counter_kinds", Label: "Menace", Player: 0},
			{Index: 1, Kind: "counter_kinds", Label: "Deathtouch", Player: 0},
			{Index: 2, Kind: "counter_kinds", Label: "Lifelink", Player: 0},
		}}
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("counter-kind bot answer %+v does not validate: %v", in, err)
	}
	if len(in.Choices) != 2 || in.Choices[0] != 0 || in.Choices[1] != 1 {
		t.Fatalf("counter-kind bot choices = %v, want first two distinct options", in.Choices)
	}
}
