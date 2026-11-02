package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// replacementMatchesDestroy is an R:Event$ Destroy line against a destruction
// proposal: ValidCard$ names the permanent, and a Regeneration$ line needs a
// permanent that can be regenerated, which excludes a NoRegen$ destruction, a
// CantRegenerate restriction and a creature with no toughness left (as Forge's
// ReplaceDestroy reads it).
func (e *Engine) replacementMatchesDestroy(r cards.Repl, source state.ObjID, ev events.Event, you state.PlayerID, svars map[string]string, remembered []state.ObjID) bool {
	if ev.Kind != events.DestroyProposal {
		return false
	}
	if v, ok := r.Param(cards.PKValidCard); ok && !e.matchesSpec(v, ev.Obj, e.rememberedSpecContext(you, source, remembered)) {
		return false
	}
	if _, ok := r.Param(cards.PKRegeneration); ok {
		if ev.Amount != 1 || e.RegenerationDisallowed(ev.Obj) {
			return false
		}
		if e.IsCreature(ev.Obj) && e.Toughness(ev.Obj) <= 0 {
			return false
		}
	}
	return e.replacementConditionHoldsWith(r, source, you, svars)
}

// continueDestroyReplacements runs the first matching line's body in place of
// the destruction. CR 616.1 gives the choice among several to the permanent's
// controller; scan order is the deterministic stand-in the shield family uses,
// and no corpus body asks.
func (e *Engine) continueDestroyReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	m := matches[0]
	if m.repl.With != nil {
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, nil)
	}
	return ev, true
}
