package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// spellSVarKeys are the keys the spell cast gate reads for CheckSVar$.
var spellSVarKeys = cards.ParamMaskOf(cards.PKCheckSVar)

// spellSVarGateOK is the spell's own CheckSVar$/SVarCompare$ cast
// restriction (Forge's SpellAbilityRestrictions.checkSVar: Reset's "Cast this
// spell only during an opponent's turn after their upkeep step" is
// CheckSVar$ Count$FinishedUpkeepsThisTurn; the Fog-likes' "only if ..." are
// CheckSVar$ X). An unmet gate withholds the offer (CR 601.3) rather than
// fizzling at resolution. Only the spell's own SA is read, as for IsPresent$.
// A gate whose count body this build cannot read fails OPEN, the ability
// gate's rule (sVarGateEval): an unreadable restriction must not remove the
// card's cast. A pure read.
func spellSVarGateOK(h effects.Host, p state.PlayerID, id state.ObjID, f *cards.Face, sa *cards.SA) bool {
	if sa == nil || f == nil || !sa.MayHaveAnyParam(spellSVarKeys) {
		return true
	}
	check, ok := sa.Param(cards.PKCheckSVar)
	if !ok {
		return true
	}
	return sVarGateEval(h, effects.NewCtxPtr(id, p, effects.CtxInit{SVars: f.SVars}), sa, check)
}
