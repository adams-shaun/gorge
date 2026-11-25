package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPreventAllTo lowers the this-turn "prevent all damage that would be
// dealt to target creature" Effect (Godtoucher, Indestructible Aura, Shielded
// Passage, Djeru's Resolve): one bodyless Event$ DamageDone | Prevent$ True |
// ValidTarget$ Card.IsRemembered line (no IsCombat$, no ValidSource$), no
// Duration$ (this turn), ended by ExileOnMoved$ Battlefield, on the one
// remembered target. One OpExt PreventNext with C 2: the unbounded shield of
// dirac/repl_prevnext.go (all the damage left, until the recipient leaves).
// It reports false, consuming nothing, for any other shape so effectPrevent's
// own arms read it (Kurbis has no ExileOnMoved$: gorge's shield follows the
// ObjID across a return).
func (lc *lowerCtx) effectPreventAllTo(ps *params) (bool, error) {
	if strings.TrimSpace(ps.peek("ExileOnMoved")) != "Battlefield" || ps.peek("Duration") != "" {
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
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("Prevent", "True") {
		return false, nil
	}
	for _, k := range []string{"IsCombat", "ValidSource", "Duration"} {
		if _, ok := rp.get(k); ok {
			return false, nil
		}
	}
	if z, ok := rp.get("ActiveZones"); ok && strings.TrimSpace(z) != "Command" {
		return false, nil
	}
	if vt := rp.str("ValidTarget"); vt != "Card.IsRemembered" && vt != "Creature.IsRemembered" {
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	ro := ps.peek("RememberObjects")
	var obj lRef
	var err error
	if _, ok := ps.m["ValidTgts"]; ok {
		if ro != "Targeted" && ro != "ThisTargetedCard" {
			return false, nil
		}
		if _, many := ps.m["TargetMax"]; many {
			return false, nil
		}
		if _, many := ps.m["TargetMin"]; many {
			return false, nil
		}
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return true, err
		}
	} else {
		if ro != "Targeted" {
			return false, nil
		}
		if obj, err = lc.defined(ro); err != nil {
			return true, err
		}
	}
	ps.consume("ReplacementEffects", "RememberObjects", "ExileOnMoved")
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: 2, Note: "prevent all damage that would be dealt to Obj this turn, until it leaves"})
	return true, nil
}

// effectPreventHost lowers Gideon's "prevent all damage that would be dealt to
// him this turn" Effect: Duration$ UntilHostLeavesPlayOrEOT over one bodyless
// Event$ DamageDone | Prevent$ True | ValidTarget$ Card.EffectSource line, the
// Effect's own source the recipient, nothing remembered or targeted. It is the
// unbounded shield effectPreventAllTo emits, on the source: it ends with the
// turn or as the source leaves the battlefield (prevAllLeft), as gorge's
// Duration$ does. It reports false, consuming nothing, for any other shape.
func (lc *lowerCtx) effectPreventHost(ps *params) (bool, error) {
	for _, k := range []string{"RememberObjects", "ValidTgts", "ExileOnMoved", "ForgetOnMoved", "StaticAbilities", "Triggers"} {
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
	rp := newParams(r.Params)
	rp.consume("Event", "Description")
	if !rp.is("Prevent", "True") || rp.str("ValidTarget") != "Card.EffectSource" {
		return false, nil
	}
	if z, ok := rp.get("ActiveZones"); ok && strings.TrimSpace(z) != "Command" {
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	obj, err := lc.defined("Self")
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "Duration")
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: 2, Note: "prevent all damage that would be dealt to this permanent this turn, until it leaves"})
	return true, nil
}
