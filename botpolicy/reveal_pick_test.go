package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestBotAnswersRevealPickLegalAndMirrorsTheStandIn pins the botpolicy arm
// for the new hand-reveal pick (infernaltutor1: effReveal's "Reveal a card
// from your hand" is a KChoose over the eligible hand cards). The bot must
// hand back an answer Decision.Validate accepts -- a rejected answer is
// re-derived identically forever (the livelock the one-home legal-answer rule
// exists to prevent) -- and the answer mirrors the engine's no-host stand-in
// (first Max options in zone order) so a bot-answered ask emits the same
// MoveZone/reveal events the silent build did.
func TestBotAnswersRevealPickLegalAndMirrorsTheStandIn(t *testing.T) {
	cases := []struct {
		name       string
		min, max   int
		wantPicks  int
		optionKind string
	}{
		// Infernal Tutor: reveal exactly one of the eligible hand cards.
		{"mandatory one-of-three", 1, 1, 1, "reveal"},
		// Mandatory n-of-m (Vizkopa Confessor's reveal 2 of 7).
		{"mandatory two-of-three", 2, 2, 2, "reveal"},
		// AnyNumber$: "reveal any number" -- the stand-in reveals all, so
		// the bot takes every eligible card (Min 0 must NOT answer "none").
		{"any number", 0, 3, 3, "reveal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &decision.Decision{
				Kind: decision.KChoose,
				Min:  tc.min,
				Max:  tc.max,
				Options: []decision.Option{
					{Index: 0, Kind: tc.optionKind, Obj: 11},
					{Index: 1, Kind: tc.optionKind, Obj: 12},
					{Index: 2, Kind: tc.optionKind, Obj: 13},
				},
			}
			in := Decide(Board{}, d, rng(1))
			if err := d.Validate(in); err != nil {
				t.Fatalf("bot answer %v failed Validate: %v", in.Choices, err)
			}
			if len(in.Choices) != tc.wantPicks {
				t.Fatalf("bot picked %v (%d), want the first %d options", in.Choices, len(in.Choices), tc.wantPicks)
			}
			for i, c := range in.Choices {
				if c != i {
					t.Fatalf("bot picked %v, want first %d in order (or an ask the engine rejects)", in.Choices, tc.wantPicks)
				}
			}
		})
	}
}
