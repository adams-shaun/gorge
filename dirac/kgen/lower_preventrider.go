package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPreventRider lowers the this-turn scoped prevention Effect whose
// replacement carries a body (gorge's PreventionEffect$ True with ReplaceWith$,
// damageReplacementPreventsWithRider): Chant of Vitu-Ghazi's "prevent all
// damage that would be dealt by creatures this turn; you gain life equal to
// the damage prevented". The line is Event$ DamageDone | PreventionEffect$
// True | ReplaceWith$ <svar> (no Prevent$), in the bit forms of
// effectPreventScope (ActiveZones$ absent or Command, IsCombat$ absent or
// True, ValidTarget$/ValidSource$ of scopeTargetBits/scopeSourceBits), and
// the body is exactly DB$ GainLife | Defined$ You | LifeAmount$ X with
// X = ReplaceCount$DamageAmount. One OpExt PreventDamageScope with
// kop.PrevScopeGain. It reports false, consuming nothing, for any other shape.
func (lc *lowerCtx) effectPreventRider(ps *params) (bool, error) {
	for _, k := range []string{"RememberObjects", "ValidTgts", "Duration"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	name := strings.TrimSpace(ps.peek("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rm := r.Params
	if _, has := rm["Prevent"]; has || strings.TrimSpace(rm["PreventionEffect"]) != "True" {
		return false, nil
	}
	if az, has := rm["ActiveZones"]; has && strings.TrimSpace(az) != "Command" {
		return false, nil
	}
	bits := kop.PrevScopeGain
	if v, has := rm["IsCombat"]; has {
		if strings.TrimSpace(v) != "True" {
			return false, nil
		}
		bits |= kop.PrevScopeCombat
	}
	tb, tok := scopeTargetBits(strings.TrimSpace(rm["ValidTarget"]))
	sb, sok := scopeSourceBits(strings.TrimSpace(rm["ValidSource"]))
	if !tok || !sok {
		return false, nil
	}
	bits |= tb | sb
	if !lc.riderBodyIsGain(strings.TrimSpace(rm["ReplaceWith"])) {
		return false, nil
	}
	rp := newParams(rm)
	rp.consume("Event", "PreventionEffect", "ActiveZones", "IsCombat", "ValidTarget", "ValidSource", "ReplaceWith")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	ps.consume("ReplacementEffects", "AILogic")
	lc.emit(lInstr{Op: "PreventDamageScope", C: bits, Note: "prevent the damage and gain that much life this turn"})
	return true, nil
}

// riderBodyIsGain reports whether SVar name is exactly DB$ GainLife | Defined$
// You | LifeAmount$ <svar> with that SVar exactly ReplaceCount$DamageAmount.
func (lc *lowerCtx) riderBodyIsGain(name string) bool {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 3 || m["DB"] != "GainLife" || m["Defined"] != "You" {
		return false
	}
	amt := m["LifeAmount"]
	return amt != "" && strings.TrimSpace(lc.face.SVars[amt]) == "ReplaceCount$DamageAmount"
}
