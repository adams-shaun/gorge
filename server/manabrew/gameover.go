package manabrew

import (
	"math"

	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// GameOverPromptID is the fixed promptId of the terminal gameOver prompt.
// The three id spaces a ManaBrew client sees are disjoint:
//
//   - ordinary decision prompts: a bare decision.Seq, always >= 0
//     (ids.go's promptID);
//   - synthetic ack-only prompts (synthetic.go): always < 0;
//   - the terminal gameOver prompt: this one constant.
//
// math.MaxInt64 is reachable by no decision.Seq a real match mints (a Seq is
// an event count, so realistic ids stay far below 2^53, and the terminal prompt's id is never echoed in an answer
// because gameOver carries no response), so the three spaces cannot collide
// in practice; the disjointness is asserted over the reachable range in
// synthetic_test.go. GameOver is delivered exactly once per seat, after the
// final state, and the
// response side already refuses any answer to it (errors.go's "gameOver
// carries no response" branch).
const GameOverPromptID int64 = math.MaxInt64

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
