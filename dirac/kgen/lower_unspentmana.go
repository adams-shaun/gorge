package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// unspentRedEOT lowers the Last Agni Kai's `DB$ Effect | StaticAbilities$
// Unspent`, `Mode$ UnspentMana | ValidPlayer$ You | ManaType$ Red`
// gorge's effEffect registers the body for its source
// until end of turn; at every step boundary finishStepBoundary keeps the
// effect controller's red (unspentManaKeep: ValidPlayer$ You, ManaType$ Red
// keeps the whole R slot) and the effect ends in the cleanup step. The
// kernel marks that with an AddMana B == 3 (no units). Only the instant/
// sorcery spelling is read (UntilEOT lifetime); every other shape reports
// (false, nil) and keeps its named refusal downstream.
func (lc *lowerCtx) unspentRedEOT(ps *params, ss []cards.Static) (bool, error) {
	if len(ss) != 1 || ss[0].Mode != "UnspentMana" || lc.ab == nil || lc.ab.Kind != akSpell {
		return false, nil
	}
	st := ss[0]
	for k := range st.Params {
		switch k {
		case "Mode", "ValidPlayer", "ManaType", "Description", "Secondary":
		default:
			return false, nil
		}
	}
	if st.Params["ValidPlayer"] != "You" || st.Params["ManaType"] != "Red" {
		return false, nil
	}
	for _, k := range []string{"Duration", "ValidTgts", "RememberObjects", "Triggers"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	ps.consume("StaticAbilities")
	lc.emit(lInstr{Op: "AddMana", Obj: lRef{Kind: rYou}, A: cst(0), B: cst(3), CS: "R", Note: "UnspentMana Red: keep red as steps end until end of turn"})
	return true, nil
}
