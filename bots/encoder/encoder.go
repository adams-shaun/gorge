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

// OptionListEncoder is optionally implemented by a ViewEncoder that can encode
// a whole option list against one view, resolving the view's cards once.
type OptionListEncoder interface {
	EncodeOptions(v *view.View, seat state.PlayerID, d decision.Kind, opts []decision.Option, dst []policynet.Option) []policynet.Option
}

// EncodeOptions encodes every option of opts through e, using e's bulk method
// when it has one. The result parallels opts.
func EncodeOptions(e ViewEncoder, v *view.View, seat state.PlayerID, d decision.Kind, opts []decision.Option, dst []policynet.Option) []policynet.Option {
	if b, ok := e.(OptionListEncoder); ok {
		return b.EncodeOptions(v, seat, d, opts, dst)
	}
	if cap(dst) < len(opts) {
		dst = make([]policynet.Option, len(opts))
	}
	dst = dst[:len(opts)]
	for i := range opts {
		dst[i] = e.EncodeOption(v, seat, d, opts[i], i, len(opts))
	}
	return dst
}
