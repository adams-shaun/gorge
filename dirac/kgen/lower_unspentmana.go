package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// unspentRedEOT lowers the Last Agni Kai's Effect, whose static keeps your
// unspent red mana (an UnspentMana static for you, mana type red):
// gorge's effEffect registers the body for its source
// until end of turn; at every step boundary finishStepBoundary keeps the
// effect controller's red (unspentManaKeep: ValidPlayer$ You, ManaType$ Red
// keeps the whole R slot) and the effect ends in the cleanup step. The
// kernel marks that with an AddMana B == 3 (no units). Only the instant/
// sorcery spelling is read (UntilEOT lifetime). The card static the S:
// lowering reads (static_unspentmana.go) is not this path. Every other
// UnspentMana shape refuses by name here: the S: body's registration took
// away the "static mode" refusal these shapes kept downstream, and the
// effect arms below (the Duration$Permanent emblem, effectStaticEOT) assume
// every static they take is one they read — an unread UnspentMana body must
// not flow into an emblem (fightexcess_test.go's contract).
func (lc *lowerCtx) unspentRedEOT(ps *params, ss []cards.Static) (bool, error) {
	if len(ss) != 1 || ss[0].Mode != "UnspentMana" {
		return false, nil
	}
	st := ss[0]
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return false, refuse("Effect UnspentMana static, spell only")
	}
	for k := range st.Params {
		switch k {
		case "Mode", "ValidPlayer", "ManaType", "Description", "Secondary":
		default:
			return false, refuse("Effect UnspentMana static %q", k)
		}
	}
	if st.Params["ValidPlayer"] != "You" || st.Params["ManaType"] != "Red" {
		return false, refuse("Effect UnspentMana static %q", st.Params["ValidPlayer"]+" "+st.Params["ManaType"])
	}
	for _, k := range []string{"Duration", "ValidTgts", "RememberObjects", "Triggers"} {
		if _, ok := ps.m[k]; ok {
			return false, refuse("Effect UnspentMana static %q", k)
		}
	}
	ps.consume("StaticAbilities")
	lc.emit(lInstr{Op: "AddMana", Obj: lRef{Kind: rYou}, A: cst(0), B: cst(3), CS: "R", Note: "UnspentMana Red: keep red as steps end until end of turn"})
	return true, nil
}
