package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryLandReturn is the filter of the land a land's own enters trigger makes
// its controller return to hand, "" for every other face. Two script shapes
// say it: a mandatory return from the battlefield ("return a land you
// control", which the entering land itself satisfies), and a sacrifice paid
// off by returning one ("sacrifice it unless you return ...").
func entryLandReturn(f *cards.Face) string {
	for i := range f.Triggers {
		t := &f.Triggers[i]
		if t.Effect == nil || t.ParamStr(cards.PKDestination) != "Battlefield" || t.ParamStr(cards.PKValidCard) != "Card.Self" {
			continue
		}
		filter := ""
		switch t.Effect.API {
		case "ChangeZone":
			filter = entryReturnFilter(t.Effect)
		case "Sacrifice":
			filter = entryUnlessReturnFilter(t.Effect)
		}
		if filter != "" {
			return filter
		}
	}
	return ""
}

// entryReturnFilter is the filter of a mandatory return of a land from the
// battlefield to hand, "" for any other zone change.
func entryReturnFilter(sa *cards.SA) string {
	if sa.ParamStr(cards.PKOrigin) != "Battlefield" || sa.ParamStr(cards.PKDestination) != "Hand" ||
		sa.ParamStr(cards.PKMandatory) != "True" || sa.ParamStr(cards.PKChangeType) != "Land.YouCtrl" {
		return ""
	}
	return "Land.YouCtrl"
}

// entryUnlessReturnFilter is the filter of the permanent a sacrifice asks to
// have returned instead, "" for a sacrifice with no such price.
func entryUnlessReturnFilter(sa *cards.SA) string {
	cost, ok := costvocab.ParseUnlessCost(sa.ParamStr(cards.PKUnlessCost))
	if !ok || len(cost.Return) != 1 {
		return ""
	}
	return cost.Return[0].Spec
}

// landWontStay reports whether land f, played by p, would not stay on the
// battlefield: its enters trigger asks for a land to return and p controls no
// other land that qualifies, so the land returns itself or is sacrificed.
func (e *Engine) landWontStay(p state.PlayerID, id state.ObjID, f *cards.Face) bool {
	filter := entryLandReturn(f)
	return filter != "" && len(pay.CostCandidates(asPayer(e), p, id, state.ZBattlefield, filter, false, false)) == 0
}
