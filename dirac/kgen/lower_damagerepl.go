package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damagePlus lowers "if <ValidSource$> would deal damage to <ValidTarget$>,
// it deals that much damage plus N instead" (DB$ ReplaceEffect of
// DamageAmount with ReplaceCount$DamageAmount/Plus.N: Valley Flamecaller;
// Artist's Talent's level 3, IsCombat$ False). ok is false when the
// replacement is not this shape (the other DamageDone shapes follow).
// gorge applies it as one more damage replacement candidate (CR 616.1:
// with two or more matching one event it asks for the order; the kernel
// stalls there, StallReplPlus).
func (lc *lowerCtx) damagePlus(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return false, nil
	}
	vv := strings.TrimSpace(w.Params["VarValue"])
	if body, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(body)
	}
	rest, ok := strings.CutPrefix(vv, "ReplaceCount$DamageAmount/Plus.")
	if !ok {
		return false, nil
	}
	kind, ctrHost, err := lc.dmgPlusCtrOperand(rest)
	if err != nil {
		return true, err
	}
	n := 0
	if !ctrHost {
		n, _ = strconv.Atoi(rest) // dmgPlusCtrOperand proved a positive integer
	}
	if rp.str("ActiveZones") != "Battlefield" {
		return true, refuse("repl-hard: DamageDone ActiveZones")
	}
	vs, ok := rp.get("ValidSource")
	if !ok {
		return true, refuse("repl-hard: DamageDone plus without ValidSource")
	}
	if rl.F, err = lc.filter(vs); err != nil {
		return true, err
	}
	if rl.F.Players != 0 || len(rl.F.Clauses) == 0 {
		return true, refuse("repl-hard: DamageDone plus ValidSource %q", vs)
	}
	for _, c := range rl.F.Clauses {
		if c.Flags&fSelf != 0 {
			return true, refuse("repl-hard: DamageDone plus ValidSource %q", vs)
		}
	}
	bits := 0
	for _, t := range strings.Split(rp.str("ValidTarget"), ",") {
		switch strings.TrimSpace(t) {
		case "Opponent", "Player.Opponent": // gorge's player spec reads both alike (Torbran, Far Fortune's granted replacement)
			bits |= kop.ReplDmgOppPlayer
		case "Permanent.OppCtrl":
			bits |= kop.ReplDmgOppPerm
		case "Player":
			bits |= kop.ReplDmgPlayer
		case "Permanent":
			bits |= kop.ReplDmgPerm
		default:
			return true, refuse("repl-hard: DamageDone plus ValidTarget %q", rp.str("ValidTarget"))
		}
	}
	if c, ok := rp.get("IsCombat"); ok {
		if c != "False" {
			return true, refuse("repl-hard: DamageDone plus IsCombat %q", c)
		}
		bits |= kop.ReplDmgNonCombat
	}
	switch {
	case ctrHost && kind == dmgPlusHostPower:
		bits |= kop.ReplDmgPowHost
	case ctrHost:
		rl.Ctr = kind
		bits |= kop.ReplDmgCtrHost
	}
	rl.Kind, rl.N, rl.Bits = "DamagePlus", cst(n), bits
	return true, nil
}
