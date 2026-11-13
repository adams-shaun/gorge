package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestSpliceBotAnswerPassesTheValidator keeps the Splice selection's answer
// rule in one home: the engine poses (rules' spliceAsk) a Min 0 / Max N
// AllowNone KChoose of Kind "splice" options; the bot declines (the empty
// answer), and that answer must pass Decision.Validate, the rule the engine's
// own Clamp enforces.
func TestSpliceBotAnswerPassesTheValidator(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 0, Max: 2, AllowNone: true,
		Prompt: "Choose cards to splice onto Glacial Ray",
		Options: []decision.Option{
			{Index: 0, Kind: "splice", Obj: 11, Label: "Kodama's Might — splice {G}"},
			{Index: 1, Kind: "splice", Obj: 12, Label: "Torrent of Stone — splice sacrifice two Mountains"},
		}}
	in := Decide(Board{}, d, rng(1))
	if len(in.Choices) != 0 {
		t.Fatalf("splice bot choices = %v, want the empty answer", in.Choices)
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("splice bot answer %+v rejected by Decision.Validate: %v", in, err)
	}
	if got := Clamp(d, in); len(got.Choices) != 0 || d.Validate(got) != nil {
		t.Fatalf("Clamp changed the bot's splice answer: %+v", got)
	}
}
