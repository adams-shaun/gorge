package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
)

// TestNightShiftBotAnswerIsLegal: the roll-modify ask's legal answer is one
// option; the bot's answer passes Validate and is option 0 (keep the die as
// rolled), the clamp default every unmodelled bot takes.
func TestNightShiftBotAnswerIsLegal(t *testing.T) {
	d := effects.RollModifyAsk(0, 1, 5)
	if len(d.Options) != 3 || d.Min != 1 || d.Max != 1 || d.Options[0].Kind != "keep" {
		t.Fatalf("precondition: unexpected ask shape %+v", d)
	}
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer %v rejected: %v", in.Choices, err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("bot answered %v, want option 0 (keep)", in.Choices)
	}
	if clamped := Clamp(d, in); len(clamped.Choices) != 1 || clamped.Choices[0] != 0 {
		t.Fatalf("clamp answered %v, want option 0", clamped.Choices)
	}
}
