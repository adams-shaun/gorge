package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestRepeatEachOptionalBotAnswerIsLegal pins the bot arm for a RepeatEach
// RepeatOptionalForEachPlayer$ election -- the distinct decision shape
// effects.poseRepeatEachElection builds: a Min/Max 1 KChoose with a "yes" and
// a "no", ResumeKind "repeat_each_optional", belonging to the SUBJECT (an
// opponent of the loop's controller), not the acting player.
//
// It has no dedicated policy arm, so it takes the shared KChoose fill
// fallback: the first offered option. The answer must round-trip through
// Decision.Validate (the engine's one legal-answer home) so the bot cannot
// re-submit a rejected answer and livelock on the next subject's election.
func TestRepeatEachOptionalBotAnswerIsLegal(t *testing.T) {
	election := func(first string) *decision.Decision {
		second := "no"
		if first == "no" {
			second = "yes"
		}
		return &decision.Decision{
			Player: 1, Kind: decision.KChoose, Min: 1, Max: 1,
			ResumeKind: "repeat_each_optional",
			Prompt:     "Do you want to create a Treasure token?",
			Options: []decision.Option{
				{Index: 0, Kind: first, Label: first, Player: 1},
				{Index: 1, Kind: second, Label: second, Player: 1},
			},
		}
	}

	// Precondition: the two offers really differ in order, so the answer
	// below reads the offered list rather than a hardcoded pick.
	acceptFirst := election("yes")
	acceptIn := Decide(Board{}, acceptFirst, rng(1))
	declineFirst := election("no")
	declineIn := Decide(Board{}, declineFirst, rng(1))
	if len(acceptIn.Choices) != 1 || len(declineIn.Choices) != 1 {
		t.Fatalf("bot answers = %v / %v, want one choice each", acceptIn.Choices, declineIn.Choices)
	}
	if acceptFirst.Options[acceptIn.Choices[0]].Kind == declineFirst.Options[declineIn.Choices[0]].Kind {
		t.Fatalf("the two offers produced the same option kind %q; the order is not binding",
			acceptFirst.Options[acceptIn.Choices[0]].Kind)
	}

	// The bot's own answer must pass the validator the engine uses.
	for _, tc := range []struct {
		name string
		d    *decision.Decision
		in   decision.Intent
	}{{"accept-first", acceptFirst, acceptIn}, {"decline-first", declineFirst, declineIn}} {
		tc.in.Player = tc.d.Player
		if err := tc.d.Validate(tc.in); err != nil {
			t.Fatalf("%s: bot answer %v rejected by Decision.Validate: %v", tc.name, tc.in.Choices, err)
		}
	}
}
