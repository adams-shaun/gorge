package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// validAfterStackCtx is the ValidAfterStack$ judge's Ctx for the targeting
// census: the activating seat p and the ability's source permanent with its
// face SVars (Keeper of the Dead's Y). nil when the ability carries no
// ValidAfterStack$, which admits every candidate.
func (e *Engine) validAfterStackCtx(spec string, source state.ObjID, p state.PlayerID) *effects.Ctx {
	if spec == "" {
		return nil
	}
	return effects.NewCtxPtr(source, p, effects.CtxInit{SVars: e.pileSVars(source, 0)})
}

// playerValidAfterStackAdmits judges one candidate seat against the ability's
// ValidAfterStack$ at the offer (CR 602.2b: the legality is read as the
// ability is activated; no recheck at resolution). A nil ctx admits.
func (e *Engine) playerValidAfterStackAdmits(ctx *effects.Ctx, spec string, q state.PlayerID) bool {
	return ctx == nil || effects.PlayerAdmitsValidAfterStack(e, ctx, spec, q)
}
