package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPrevent lowers a DB$ Effect whose ReplacementEffects$ are gorge's
// bodyless combat-damage prevention lines on the remembered creature
// (effects/effect.go's Prevent$ True DamageDone registration: a this-turn
// grant when no Duration$ is given): Prevent$ True | IsCombat$ True |
// ValidTarget$ (prevented to it) or ValidSource$ (prevented from it)
// Card.IsRemembered. Lowered as an until-end-of-turn keyword grant on the
// remembered object (KwPreventCombatDamageTo / KwPreventCombatDamageFrom):
// the grant ends when the object leaves, as ExileOnMoved$/ForgetOnMoved$
// Battlefield end gorge's. Every other replacement shape refuses.
func (lc *lowerCtx) effectPrevent(ps *params) error {
	if ok, err := lc.effectEtbCounters(ps); ok || err != nil { // lower_effetbctr.go
		return err
	}
	if ok, err := lc.effectStackExile(ps); ok || err != nil { // lower_effectstackexile.go
		return err
	}
	if ok, err := lc.effectCounterPlusEOT(ps); ok || err != nil { // lower_ctrpluseot.go
		return err
	}
	if ps.is("Duration", "UntilTheEndOfYourNextUntap") {
		return lc.skipUntapEffect(ps)
	}
	if ps.is("Duration", "AsLongAsControl") {
		if ok, err := lc.untapLockEffect(ps); ok || err != nil { // lower_untaplock.go
			return err
		}
	}
	if ps.is("Duration", "UntilYourNextTurn") {
		return lc.effectDmgMark(ps) // lower_effectdmgmark.go
	}
	if _, ok := ps.get("Duration"); ok {
		return refuse("Effect replacement Duration$")
	}
	if ok, err := lc.effectPreventScope(ps); ok || err != nil {
		return err
	}
	for _, k := range []string{"ExileOnMoved", "ForgetOnMoved"} {
		if v, ok := ps.get(k); ok && strings.TrimSpace(v) != "Battlefield" {
			return refuse("Effect replacement %s$ %q", k, v)
		}
	}
	var kws []string
	for _, name := range strings.Split(ps.str("ReplacementEffects"), ",") {
		r, ok := cards.ParseReplacementLine(lc.face.SVars[strings.TrimSpace(name)])
		if !ok || r.Event != "DamageDone" {
			return refuse("Effect replacement %q", strings.TrimSpace(name))
		}
		rp := newParams(r.Params)
		rp.consume("Event", "ActiveZones")
		if !rp.is("Prevent", "True") || !rp.is("IsCombat", "True") {
			return refuse("Effect replacement shape")
		}
		vt, vs := rp.str("ValidTarget"), rp.str("ValidSource")
		switch {
		case vt == "Card.IsRemembered" && vs == "":
			kws = append(kws, "PreventCombatDamageTo")
		case vs == "Card.IsRemembered" && vt == "":
			kws = append(kws, "PreventCombatDamageFrom")
		default:
			return refuse("Effect replacement shape")
		}
		if left := rp.unread(); len(left) > 0 {
			return refuse("Effect replacement unread %v", left)
		}
	}
	ro := ps.str("RememberObjects")
	var obj lRef
	var err error
	if _, ok := ps.get("ValidTgts"); ok {
		if ro != "Targeted" && ro != "ThisTargetedCard" {
			return refuse("Effect RememberObjects %q", ro)
		}
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	} else {
		if ro != "Targeted" && ro != "Self" {
			return refuse("Effect RememberObjects %q", ro)
		}
		if obj, err = lc.defined(ro); err != nil {
			return err
		}
	}
	lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(kws, "&")})
	return nil
}

// effectPreventScope lowers the this-turn prevention Effect that names no
// remembered object: one bodyless Event$ DamageDone | Prevent$ True line in
// the Command zone whose ValidTarget$ is You, Creature.YouCtrl or both, with
// ValidSource$ Creature.inZoneBattlefield or none, and no IsCombat$ (combat
// and noncombat alike). One OpExt PreventDamageScope. It reports false for
// any other shape so effectPrevent's own arms read it.
func (lc *lowerCtx) effectPreventScope(ps *params) (bool, error) {
	if _, ok := ps.get("RememberObjects"); ok {
		return false, nil
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return false, nil
	}
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("Prevent", "True") || !rp.is("ActiveZones", "Command") {
		return false, nil
	}
	if _, ok := rp.get("IsCombat"); ok {
		return false, nil
	}
	bits := 0
	for _, w := range strings.Split(rp.str("ValidTarget"), ",") {
		switch strings.TrimSpace(w) {
		case "You":
			bits |= kop.PrevScopePlayer
		case "Creature.YouCtrl":
			bits |= kop.PrevScopeCreatures
		default:
			return false, nil
		}
	}
	if bits == 0 {
		return false, nil
	}
	switch rp.str("ValidSource") {
	case "":
	case "Creature.inZoneBattlefield":
		bits |= kop.PrevScopeSrcCreature
	default:
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("Effect replacement unread %v", left)
	}
	ps.consume("ReplacementEffects", "AILogic")
	lc.emit(lInstr{Op: "PreventDamageScope", C: bits, Note: "prevent the damage the controller and their creatures would take this turn"})
	return true, nil
}
