package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestRepeatOptionalBotAnswerIsLegal pins the RepeatOptional$ election's bot
// arm (policy.go's `d.ResumeKind == "repeat_optional"` branch): it repeats
// while the acting player's life is above the deterministic safety margin and
// stops once it is not, and every answer it hands back passes
// Decision.Validate. The decision shape is the one
// effects.poseRepeatOptionalElection builds -- a Min/Max 1 KChoose with a
// "yes" and a "no".
//
// The legal-answer rule has ONE home: Decision.Validate (via Clamp's
// FitRequired) is what the engine will accept, so the bot's own answer for
// this kind must round-trip through it. A future tightening of the election's
// arity or option set that the bot does not honour is caught here rather than
// as a livelock.
func TestRepeatOptionalBotAnswerIsLegal(t *testing.T) {
	election := func() *decision.Decision {
		return &decision.Decision{
			Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
			ResumeKind: "repeat_optional", Prompt: "Repeat this process?",
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Repeat", Player: 0},
				{Index: 1, Kind: "no", Label: "Stop", Player: 0},
			},
		}
	}

	// Precondition: the two arms really differ, so the assertions below can
	// tell "repeat" from "stop" and neither is a vacuous pass.
	high := election()
	highIn := Decide(Board{Life: map[state.PlayerID]int32{0: 20}}, high, rng(1))
	if len(highIn.Choices) != 1 || highIn.Choices[0] != 0 {
		t.Fatalf("high life answer = %v, want [0] (repeat)", highIn.Choices)
	}
	low := election()
	lowIn := Decide(Board{Life: map[state.PlayerID]int32{0: 5}}, low, rng(1))
	if len(lowIn.Choices) != 1 || lowIn.Choices[0] != 1 {
		t.Fatalf("low life answer = %v, want [1] (stop)", lowIn.Choices)
	}
	if highIn.Choices[0] == lowIn.Choices[0] {
		t.Fatalf("the two boards produced the same answer %v; the life gate is not binding", highIn.Choices)
	}

	// The bot's own answer must pass the validator the engine uses.
	for _, tc := range []struct {
		name string
		d    *decision.Decision
		in   decision.Intent
	}{{"high", high, highIn}, {"low", low, lowIn}} {
		if err := tc.d.Validate(tc.in); err != nil {
			t.Fatalf("%s: bot answer %v rejected by Decision.Validate: %v", tc.name, tc.in.Choices, err)
		}
	}
}
