package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lifeLossRepl lowers Bloodletter of Aclazotz's life-loss doubler (gorge
// applyLifeReplacements' LifeReduced arm, cr-lifereduced-twice-not-damage
// d86d838e0): Event$ LifeReduced | ValidPlayer$ Opponent | PlayerTurn$ True |
// ActiveZones$ Battlefield, its body DB$ ReplaceEffect | VarName$ Amount |
// VarValue$ X with X = ReplaceCount$Amount/Twice. Every other
// LifeReduced shape refuses.
func (lc *lowerCtx) lifeLossRepl(rp *params, w *cards.SA, rl *lRepl) error {
	rp.consume("AILogic")
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidPlayer") != "Opponent" || !rp.is("PlayerTurn", "True") || w == nil ||
		!withIs(w, "ReplaceEffect", map[string]string{"VarName": "Amount", "VarValue": w.Params["VarValue"]}) ||
		w.Params["VarValue"] != "ReplaceCount$Amount/Twice" && lc.face.SVars[w.Params["VarValue"]] != "ReplaceCount$Amount/Twice" {
		return refuse("repl-hard: LifeReduced replacement")
	}
	rl.Kind = "LifeLossDouble"
	return nil
}

// lifeGainRepl lowers the life-gain replacements of the controller (gorge
// applyLifeReplacements' ReplaceCount$ grammar): Event$ GainLife |
// ActiveZones$ Battlefield | ValidPlayer$ You, its body DB$ ReplaceEffect |
// VarName$ LifeGained | VarValue$ X with X = ReplaceCount$LifeGained/Twice
// (LifeDouble) or /Plus.N (LifePlus). Opponents', prevention and
// lose-instead forms refuse.
func (lc *lowerCtx) lifeGainRepl(rp *params, w *cards.SA, rl *lRepl) error {
	rp.consume("AILogic")
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidPlayer") != "You" || w == nil ||
		!withIs(w, "ReplaceEffect", map[string]string{"VarName": "LifeGained", "VarValue": w.Params["VarValue"]}) {
		return refuse("repl-hard: GainLife replacement")
	}
	switch v := lc.face.SVars[w.Params["VarValue"]]; {
	case v == "ReplaceCount$LifeGained/Twice":
		rl.Kind = "LifeDouble"
	case strings.HasPrefix(v, "ReplaceCount$LifeGained/Plus."):
		n, err := strconv.Atoi(strings.TrimPrefix(v, "ReplaceCount$LifeGained/Plus."))
		if err != nil || n <= 0 {
			return refuse("repl-hard: GainLife replacement %q", v)
		}
		rl.Kind, rl.N = "LifePlus", cst(n)
	default:
		return refuse("repl-hard: GainLife replacement %q", v)
	}
	return nil
}
