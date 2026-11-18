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
// X = ReplaceCount$DamageAmount. A Duration$ UntilEndOfCombat on the Effect
// marks the record kop.PrevScopeEndCombat instead of the turn (prevstate-04's
// shapes). One OpExt PreventDamageScope with kop.PrevScopeGain. It reports
// false, consuming nothing, for any other shape.
func (lc *lowerCtx) effectPreventRider(ps *params, eoc bool) (bool, error) {
	for _, k := range []string{"RememberObjects", "ValidTgts"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	if _, ok := ps.get("Duration"); ok && !eoc {
		return false, nil
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
	if eoc {
		bits |= kop.PrevScopeEndCombat
	}
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
	if eoc {
		ps.consume("Duration", "UntilEndOfCombat")
	}
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

// riderBodyIsGainSub is riderBodyIsGain with one more key: SubAbility$ <sub>,
// whose SVar is exactly the Effect's self-exile (Awe Strike's ExileEffect,
// DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile): the
// body runs once and the Effect exiles itself.
func (lc *lowerCtx) riderBodyIsGainSub(name string) bool {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 4 || m["DB"] != "GainLife" || m["Defined"] != "You" {
		return false
	}
	amt := m["LifeAmount"]
	if amt == "" || strings.TrimSpace(lc.face.SVars[amt]) != "ReplaceCount$DamageAmount" {
		return false
	}
	sub := etbParams(lc.face.SVars[m["SubAbility"]])
	return len(sub) == 4 && sub["DB"] == "ChangeZone" && sub["Defined"] == "Self" &&
		sub["Origin"] == "Command" && sub["Destination"] == "Exile"
}

// effectPreventRiderFrom lowers the Awe Strike shape: an Effect on one
// remembered target creature (ValidTgts$ Creature, RememberObjects$ Targeted,
// ExileOnMoved$ Battlefield, no Duration$) whose DamageDone replacement is
// keyed on the source (ValidSource$ Card.IsRemembered, no ValidTarget$, no
// Prevent$), carries PreventionEffect$ True and ReplaceWith$ <svar>, and
// whose body is the prevstate-01 GainLife form plus the self-exile sub
// (riderBodyIsGainSub): "the next time target creature would deal damage
// this turn, prevent that damage. You gain life equal to the damage
// prevented this way" (CR 615.5, 615.8). IsCombat$ absent matches combat and
// noncombat alike; True matches combat only. It emits one Ext PreventNext
// with kop.PrevNextAll|kop.PrevNextFrom|kop.PrevNextGain (and PrevNextCombat
// for IsCombat$ True): the source-keyed shield of dirac/repl_prevnext.go,
// spent by the first event from the creature (the body's self-exile); the
// creature leaving the battlefield first ends it by ExileOnMoved$ (the
// unbounded shield's prevAllLeft). A remembered-source line with no
// self-exile sub (an "every time" form) returns false, nil with nothing
// consumed.
func (lc *lowerCtx) effectPreventRiderFrom(ps *params) (bool, error) {
	if strings.TrimSpace(ps.peek("ExileOnMoved")) != "Battlefield" || ps.peek("Duration") != "" {
		return false, nil
	}
	if strings.TrimSpace(ps.peek("ValidTgts")) != "Creature" || ps.peek("RememberObjects") != "Targeted" {
		return false, nil
	}
	if _, many := ps.m["TargetMax"]; many {
		return false, nil
	}
	if _, many := ps.m["TargetMin"]; many {
		return false, nil
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
	if _, has := rm["ValidTarget"]; has {
		return false, nil
	}
	if vs := strings.TrimSpace(rm["ValidSource"]); vs != "Card.IsRemembered" {
		return false, nil
	}
	bits := kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextGain
	if v, has := rm["IsCombat"]; has {
		if strings.TrimSpace(v) != "True" {
			return false, nil
		}
		bits |= kop.PrevNextCombat
	}
	if !lc.riderBodyIsGainSub(strings.TrimSpace(rm["ReplaceWith"])) {
		return false, nil
	}
	rp := newParams(rm)
	rp.consume("Event", "PreventionEffect", "ValidSource", "IsCombat", "ReplaceWith", "Description")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "AILogic", "ValidTgts", "RememberObjects", "ExileOnMoved", "Name")
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: bits, Note: "the next damage the remembered creature deals is prevented and its controller gains that much life"})
	return true, nil
}
