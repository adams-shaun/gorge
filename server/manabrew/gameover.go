package manabrew

import (
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// GameOverPromptID is the fixed promptId of the terminal gameOver prompt: the
// top of the u32 range (ids.go splits the range three ways, disjoint by
// construction). The terminal prompt's id is never echoed in an answer
// because gameOver carries no response. GameOver is delivered exactly once
// per seat, after the final state, and the response side already refuses any
// answer to it (errors.go's "gameOver carries no response" branch).
const GameOverPromptID int64 = PromptIDLimit - 1

// GameOver builds the terminal gameOver prompt for the seat whose view v is
// handed in: DecidingPlayerID is that seat (every seat of the table gets its
// own copy -- exactly one, delivered by the transport, after the final state
// message). The input is the protocol's only no-response prompt type
// (GameOverInput), so there is nothing to populate beyond the base.
func (t *Translator) GameOver(v *view.View) mb.PromptMessage {
	return mb.PromptMessage{Kind: "prompt", AgentPrompt: mb.AgentPrompt{
		PromptID:         GameOverPromptID,
		DecidingPlayerID: playerID(v.Viewer),
		Input:            mb.PromptInput{Value: mb.GameOverInput{}},
	}}
}
