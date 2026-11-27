package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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
	if ok, err := lc.effectPreventAllToCtr(ps); ok || err != nil { // the counter-rider form below
		return ok, err
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

// effectPreventAllToCtr lowers the unbounded this-turn shield whose
// replacement carries a counter body (Brace for Impact, Gatta and Luzzu):
// effectPreventAllTo's shape plus ReplaceWith$ <svar> on the line, whose body
// is exactly DB$ PutCounter | Defined$ ReplacedTarget | CounterType$ P1P1 |
// CounterNum$ <svar> with that SVar exactly ReplaceCount$DamageAmount
// (riderBodyIsCounter). The line is Prevent$ True or PreventionEffect$ True;
// the C bits follow gorge's own classifier (rules/replacement_damage.go's
// damageReplacementPreventsWithRider): only the PreventionEffect$ True line
// that is not itself a Prevent$ True prevention runs its body, so only that
// one emits kop.PrevNextRiderCounter beside kop.PrevNextAll (Brace for
// Impact, prevNextTake hands the whole event to prevNextRiderApply; W unset:
// prevRiderSlot reads the record's recipient, the remembered creature). A
// Prevent$ True line with a body (Gatta and Luzzu) is the plain unbounded
// shield: gorge's Prevent$ True arm terminates the event before the body
// runs, so the body is read here and dropped, exactly as gorge drops it. It
// reports false, consuming nothing, for any other shape so
// effectPreventAllTo's bodyless arm and effectPrevent's own refusals read it.
func (lc *lowerCtx) effectPreventAllToCtr(ps *params) (bool, error) {
	if _, ok := ps.m["ValidTgts"]; !ok {
		return false, nil
	}
	ro := ps.peek("RememberObjects")
	if ro != "Targeted" && ro != "ThisTargetedCard" {
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
	rp := newParams(r.Params)
	rp.consume("Event")
	// gorge's own classification (rules/replacement_damage.go): a Prevent$
	// True line is a prevention whose terminal arm never reaches the body,
	// and damageReplacementPreventsWithRider requires the line not to be one.
	// Only the PreventionEffect$ True line therefore runs its counter body.
	if !rp.is("Prevent", "True") && !rp.is("PreventionEffect", "True") {
		return false, nil
	}
	rider := rp.is("PreventionEffect", "True") && !rp.is("Prevent", "True")
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
	if !lc.riderBodyIsCounter(rp.str("ReplaceWith")) {
		return false, nil
	}
	rp.consume("Prevent", "PreventionEffect", "ActiveZones", "ValidTarget", "ReplaceWith", "Description")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "RememberObjects", "ExileOnMoved", "AILogic")
	c, note := kop.PrevNextAll, "prevent all damage that would be dealt to Obj this turn (gorge drops the counter body of a Prevent$ True line)"
	if rider {
		c |= kop.PrevNextRiderCounter
		note = "prevent all damage that would be dealt to Obj this turn and put that many +1/+1 counters on it"
	}
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: c, Note: note})
	return true, nil
}

// riderBodyIsCounter reports whether SVar name is exactly DB$ PutCounter |
// Defined$ ReplacedTarget | CounterType$ P1P1 | CounterNum$ <svar> with that
// SVar exactly ReplaceCount$DamageAmount (Brace for Impact's ImpactCounters,
// Gatta and Luzzu's Counters): the rider puts that many +1/+1 counters on the
// prevented event's target at each application.
func (lc *lowerCtx) riderBodyIsCounter(name string) bool {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 4 || m["DB"] != "PutCounter" || m["Defined"] != "ReplacedTarget" ||
		m["CounterType"] != "P1P1" {
		return false
	}
	amt := m["CounterNum"]
	return amt != "" && strings.TrimSpace(lc.face.SVars[amt]) == "ReplaceCount$DamageAmount"
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
