package effects

// This file is api:Clone's conditional-keyword rider (CloneParams.IfNew,
// Flesh Duplicate's `AddKeywords$ IfNew Vanishing:3` -- "vanishing 3 if that
// creature doesn't have vanishing", CR 707.9b's copy exception). The
// whitelist (cloneETBShapeOK) admits only riders whose inner keyword is a
// single-word head cards can expand on demand; this file evaluates the
// condition against the copy source's printed face and names the grant the
// ClonePermanent event carries.
//
// The grant is deliberately folded onto the COPY BASIS FACE, not registered
// as a layer-6 continuous effect: the basis face is what every reader (name,
// keywords, triggers, replacements) already consults, so the granted
// keyword's printed expansion -- its entry-counter replacement, its upkeep
// and last-counter triggers -- rides the ordinary printed pipelines with no
// new machinery, and it clears with the basis on a zone change (CR 400.7),
// exactly the exception's lifetime. events.Apply's ClonePermanent fold
// applies the payload through cards.Face.ApplyKeywordExpansion, so a replay
// rebuilds the identical face from the identical event.

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// cloneIfNewPayload evaluates a Clone's conditional keyword riders against
// the copy source's printed face and returns the granted keyword lines
// semicolon-joined for the ClonePermanent event's "ifnew " Counter payload.
// Empty when the clone carries no riders (the overwhelmingly common case, a
// len check), when the condition grants nothing -- the copied template
// already carries the keyword's head, so the exception does not apply -- or
// when the source object has left (its pair is skipped by the copy loop).
func cloneIfNewPayload(h Host, c *Ctx, cp *CloneParams, src state.ObjID) string {
	if len(cp.IfNew) == 0 {
		return ""
	}
	o := h.Game().Obj(src)
	if o == nil || o.CopiableFace() == nil {
		return ""
	}
	f := o.CopiableFace()
	granted := make([]string, 0, len(cp.IfNew))
	for _, kw := range cp.IfNew {
		// CR 707.9b compares the copied template's COPIABLE characteristics,
		// i.e. its printed face -- the exact face the ClonePermanent fold
		// snapshots. A keyword this build granted through a layer (a
		// continuous effect's AddKeywords) is not the template's own
		// characteristic and does not satisfy the condition, matching the
		// fold, which only ever sees the printed face.
		if !f.HasKeyword(cards.KeywordHead(kw)) {
			granted = append(granted, kw)
		}
	}
	return strings.Join(granted, ";")
}
