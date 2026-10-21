package pay

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// ForageOptions builds the either-or forage election every payment path
// poses (CR 701.61a): exile three graveyard cards when the payer has them,
// then one option per controlled Food. The caller owns the Decision; an empty
// result means the election is no longer payable.
func ForageOptions(e Engine, p state.PlayerID, source state.ObjID) []decision.Option {
	var opts []decision.Option
	if len(e.Game().Zone(state.ZGraveyard, p)) >= 3 {
		opts = append(opts, decision.Option{Index: len(opts), Kind: "forage_exile",
			Label: "Exile three cards from your graveyard"})
	}
	for _, id := range CostCandidates(e, p, source, state.ZBattlefield, "Food.YouCtrl", false, false) {
		opts = append(opts, decision.Option{Index: len(opts), Kind: "forage_food", Obj: id,
			Label: "Sacrifice " + TargetName(e.Game(), id)})
	}
	return opts
}
