package encoder

import (
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// ViewEncoder is the gorge front end of an encoder: it reads a projected
// view.View for a seat. Its output keeps the policynet layout; the contract
// is the Schema.
type ViewEncoder interface {
	Schema() Schema
	// EncodeState encodes v for seat into buf's storage (nil buf allocates).
	// diag is read only by a Hidden encoder (nil = none known).
	EncodeState(buf *policynet.StateBuf, v *view.View, seat state.PlayerID, diag *Diag) policynet.State
	// EncodeOption encodes option o (index idx of n) of a decision of kind d.
	EncodeOption(v *view.View, seat state.PlayerID, d decision.Kind, o decision.Option, idx, n int) policynet.Option
}
