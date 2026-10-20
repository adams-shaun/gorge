package manabrewhttp

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// submitProtocolError classifies a Registry.SubmitIntent error for the
// ManaBrew transport. The decision d is the one the translated intent
// answered. A refusal that leaves that same decision pending is the engine's
// verdict on an otherwise well-formed answer (the host re-parks the decision
// for another try), so it is reported as invalidShape with the engine's
// reason. Anything else means the live decision moved between the transport's
// fetch and the submit, which is a genuine stale prompt.
func (h *handler) submitProtocolError(t host.TableID, k int, seat state.PlayerID, d *decision.Decision, err error) mb.ProtocolError {
	pe := mb.ProtocolError{Code: mb.CodeStalePrompt, Message: err.Error()}
	if d == nil {
		return pe
	}
	id := int64(d.Seq)
	pe.PromptID = &id
	again, perr := h.reg.Pending(t, k, seat)
	if perr == nil && again.Seq == d.Seq {
		pe.Code = mb.CodeInvalidShape
		pe.Message = "engine refused the intent: " + err.Error()
	}
	return pe
}

// submitIntent submits one translated intent, letting a test substitute the
// registry call. The substitution exists because a stream auto-pass is always
// a legal pass: the live engine can never refuse it, so the stream's refusal
// handling has no other deterministic fixture.
func (h *handler) submitIntent(t host.TableID, k int, seat state.PlayerID, in decision.Intent) error {
	if h.submitIntentFn != nil {
		return h.submitIntentFn(t, k, seat, in)
	}
	return h.reg.SubmitIntent(t, k, seat, in)
}
