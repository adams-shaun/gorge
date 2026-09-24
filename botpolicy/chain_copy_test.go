package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// CH1: the bot accepts the first link of a may-copy chain (the original
// spell) and declines a copy_optional election whose spell is itself a copy
// (decision.Decision.CopyOfCopy), so two bots cannot hand Chain of Smog back
// and forth until the intent cap (cardfuzz batch1 lines 13/15/20).
func TestChainCopyDeclinesCopyOfCopy(t *testing.T) {
	election := func(copyOfCopy bool) *decision.Decision {
		return &decision.Decision{Seq: 4, Player: 1, Kind: decision.KChoose, Min: 1, Max: 1,
			ResumeKind: "copy_optional", CopyOfCopy: copyOfCopy,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes — copy", Player: 1},
				{Index: 1, Kind: "no", Label: "No", Player: 1},
			}}
	}
	b := Board{}
	first := election(false)
	if in := Decide(b, first, rng(1)); first.Options[in.Choices[0]].Kind != "yes" {
		t.Errorf("first link = %+v, want yes (the copy of the original keeps the card's value)", in)
	}
	cont := election(true)
	if in := Decide(b, cont, rng(1)); cont.Options[in.Choices[0]].Kind != "no" {
		t.Errorf("copy of a copy = %+v, want no (the chain would continue forever)", in)
	}
}
