package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestAssistBotAnswerValidates keeps the CR 702.132 Assist election's
// one-home answer rule honest. The decision the engine poses (rules'
// assistDecision) is a Min 1 / Max 1 KChoose asked of the ASSISTING seat, one
// option per payable amount with the amount riding Option.Amount and every
// option Kind "assist". The deterministic bot declines (the 0 option), so it
// never spends its own mana on another seat's spell, and the same answer must
// pass Decision.Validate — the shared rule the bot's KChoose arm and the
// flow's castAnswer arm both dispatch on. A parallel re-implementation that
// picked an option the generic contract rejects would livelock the bot, so
// this test fails if that ever happens. The reordered list below pins that the
// decline is taken by KIND and AMOUNT, never by position.
func TestAssistBotAnswerValidates(t *testing.T) {
	for _, opts := range [][]decision.Option{
		{
			{Index: 0, Kind: "assist", Player: 2, Label: "Pay nothing toward the cost", Amount: 0},
			{Index: 1, Kind: "assist", Player: 2, Label: "Pay {1}", Amount: 1},
			{Index: 2, Kind: "assist", Player: 2, Label: "Pay {2}", Amount: 2},
			{Index: 3, Kind: "assist", Player: 2, Label: "Pay {3}", Amount: 3},
		},
		{
			{Index: 0, Kind: "assist", Player: 2, Label: "Pay {3}", Amount: 3},
			{Index: 1, Kind: "assist", Player: 2, Label: "Pay {2}", Amount: 2},
			{Index: 2, Kind: "assist", Player: 2, Label: "Pay {1}", Amount: 1},
			{Index: 3, Kind: "assist", Player: 2, Label: "Pay nothing toward the cost", Amount: 0},
		},
	} {
		d := &decision.Decision{Seq: 1, Player: 2, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt:  "Pay how much toward Bring Down?",
			Options: opts}
		in := Decide(Board{}, d, rng(1))
		if len(in.Choices) != 1 {
			t.Fatalf("assist bot choices = %v, want exactly one option", in.Choices)
		}
		if got := d.Options[in.Choices[0]]; got.Kind != "assist" || got.Amount != 0 {
			t.Fatalf("assist bot chose %+v, want the Amount 0 assist option (the deterministic decline)", got)
		}
		if err := d.Validate(in); err != nil {
			t.Fatalf("assist bot answer %+v rejected by Decision.Validate: %v", in, err)
		}
	}
}
