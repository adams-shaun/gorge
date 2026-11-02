package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damageRaise lowers Ojer Axonil's "If a red source you control would deal
// an amount of noncombat damage less than NICKNAME's power to an opponent,
// that source deals damage equal to NICKNAME's power instead":
// R:Event$ DamageDone | DamageAmount$ LTX | ValidSource$ Card.RedSource+YouCtrl
// | IsCombat$ False | ValidTarget$ Opponent, with DB$ ReplaceEffect of
// DamageAmount to X = Count$CardPower. Only that exact shape lowers. The
// kernel applies it in replPlus as ReplDamagePlus with ReplDmgRaise; with
// another damage replacement matching the event gorge asks for the order
// (CR 616.1), and the kernel stalls (StallReplPlus). ok is false when the
// replacement is not this shape.
func (lc *lowerCtx) damageRaise(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return false, nil
	}
	if rp.str("DamageAmount") != "LTX" || lc.face.SVars[w.Params["VarValue"]] != "Count$CardPower" || lc.face.SVars["X"] != "Count$CardPower" {
		return false, nil
	}
	if w.Params["VarValue"] != "X" || rp.str("ActiveZones") != "Battlefield" {
		return true, refuse("repl-hard: DamageDone raise shape")
	}
	if rp.str("ValidSource") != "Card.RedSource+YouCtrl" || rp.str("ValidTarget") != "Opponent" || !rp.is("IsCombat", "False") {
		return true, refuse("repl-hard: DamageDone raise ValidSource %q ValidTarget %q", rp.str("ValidSource"), rp.str("ValidTarget"))
	}
	var err error
	// gorge wordColourSource: "a red source" is the object carrying red
	// (ColorsOf), which is the Red predicate for a permanent or a spell
	if rl.F, err = lc.filter("Card.Red+YouCtrl"); err != nil {
		return true, err
	}
	rl.Kind, rl.N, rl.Bits = "DamagePlus", cst(0), kop.ReplDmgOppPlayer|kop.ReplDmgNonCombat|kop.ReplDmgRaise
	return true, nil
}

// selfDiesTrigger reports whether the ability being lowered is the effect of
// a "When this dies" trigger: its triggered card is then the host itself in
// its graveyard, and a Transformed$ return of it enters the back face (the
// kernel's enterTransformed turns the one transforming double-faced card).
func (lc *lowerCtx) selfDiesTrigger() bool {
	tr := lc.ab.Trigger
	return tr != nil && tr.Event == "Dies" && tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0
}
