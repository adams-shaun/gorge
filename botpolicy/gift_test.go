package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestGiftPromiseBotAnswerValidates keeps the Gift election's one-home answer
// rule honest: the decision the engine poses (rules' giftAsk) is a Min 1 /
// Max 1 KChoose whose decline option is Kind "gift_decline" and whose promise
// options are Kind "gift_promise". The deterministic bot takes the decline by
// KIND (the plain-cast direction), and the same answer must pass
// Decision.Validate -- the shared rule both the bot's KChoose arm and the
// flow's castAnswer arm dispatch on. A parallel re-implementation that picked
// an option the generic contract rejects would livelock the bot; this test
// fails if that ever happens.
func TestGiftPromiseBotAnswerValidates(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Promise a gift?",
		Options: []decision.Option{
			{Index: 0, Kind: "gift_promise", Player: 1, Label: "Promise seat b a gift"},
			{Index: 1, Kind: "gift_decline", Label: "Don't promise a gift"},
		}}
	in := Decide(Board{}, d, rng(1))
	if len(in.Choices) != 1 {
		t.Fatalf("gift bot choices = %v, want exactly one option", in.Choices)
	}
	if got := d.Options[in.Choices[0]].Kind; got != "gift_decline" {
		t.Fatalf("gift bot chose option kind %q, want gift_decline (the deterministic decline)", got)
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("gift bot answer %+v rejected by Decision.Validate: %v", in, err)
	}
}
