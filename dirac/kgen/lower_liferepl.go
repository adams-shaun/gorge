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
	if ok, err := lc.lifeFloorRepl(rp, w, rl); ok || err != nil { // below
		return err
	}
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

// lifeFloorRepl lowers the damage floor (gorge applyLifeReplacements'
// LifeReduced arm): Event$ LifeReduced | ActiveZones$ Battlefield |
// ValidPlayer$ You.lifeGE<n> | Result$ LT<n> | IsDamage$ True, its body
// DB$ ReplaceEffect | VarName$ Amount | VarValue$ X with X =
// ReplaceCount$Amount/LimitMax.<d> and <d> = Count$YourLifeTotal/Minus.n —
// damage that would reduce the player's life total below n while they have n
// or more keeps it at n (Ali from Cairo, Elderscale Wurm, Worship; gorge
// replacement.go's LifeReduced arm matches the event as the amount becomes
// the clamped one). The ValidPlayer$ spec is the mode's own read: its
// lifeGE compare is the floor's gate, not a player scope, so the shared
// modeParam arm (plain You) stays untouched — the additive rule. IsPresent$
// is the generic presence gate (Worship). IsDamage$ True is required: a life
// cost is paid, never floored (CR 119.4). false: not this shape.
func (lc *lowerCtx) lifeFloorRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	vp, ok := rp.get("ValidPlayer")
	if !ok || !strings.HasPrefix(vp, "You.lifeGE") {
		return false, nil
	}
	if rp.str("ActiveZones") != "Battlefield" || !rp.is("IsDamage", "True") ||
		w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "Amount", "VarValue": w.Params["VarValue"]}) {
		return true, refuse("repl-hard: LifeReduced IsDamage %q shape", rp.str("IsDamage"))
	}
	n, errN := strconv.Atoi(strings.TrimPrefix(vp, "You.lifeGE"))
	lt, hasLT := rp.get("Result")
	m, errM := strconv.Atoi(strings.TrimPrefix(lt, "LT"))
	if errN != nil || errM != nil || !hasLT || n < 1 || n > 1<<14 || n != m {
		return true, refuse("repl-hard: LifeReduced floor life %d/%d", n, m)
	}
	body := strings.TrimSpace(lc.face.SVars[w.Params["VarValue"]])
	diffName, okDiff := strings.CutPrefix(body, "ReplaceCount$Amount/LimitMax.")
	dv := ""
	if okDiff {
		dv = strings.TrimSpace(lc.face.SVars[diffName])
	}
	dn, errD := 0, error(nil)
	if dn, errD = strconv.Atoi(strings.TrimPrefix(dv, "Count$YourLifeTotal/Minus.")); errD != nil || dn != n {
		return true, refuse("repl-hard: LifeReduced floor body %q", body)
	}
	bp := newParams(w.Params)
	bp.consume("VarName", "VarValue")
	if left := bp.unread(); len(left) > 0 {
		return true, refuse("repl-hard: LifeReduced floor body params %v", left)
	}
	rl.Kind, rl.N = "LifeFloor", cst(n)
	if ip, ok := rp.get("IsPresent"); ok {
		f, err := lc.filter(ip)
		if err != nil {
			return true, err
		}
		rl.Cond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	}
	return true, nil
}
