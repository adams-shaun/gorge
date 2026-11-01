package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// landWontStay reports whether land f, played by p, would not stay on the
// battlefield: its enters trigger asks for a land to return and p controls no
// other land that qualifies, so the land returns itself or is sacrificed.
func (e *Engine) landWontStay(p state.PlayerID, id state.ObjID, f *cards.Face) bool {
	filter := effects.EntryLandReturn(f)
	return filter != "" && len(pay.CostCandidates(asPayer(e), p, id, state.ZBattlefield, filter, false, false)) == 0
}
