package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPlaneCurse lowers Eloren Wilds' plane-source curse: a DB$ Effect
// whose StaticAbilities$ is exactly one CantBeCast static over the player the
// Effect targeted and whose Triggers$ is exactly one PlaneswalkedTo trigger
// whose Execute$ body is the ChangeZone Self Command->Exile self-exile idiom.
//
// gorge reads the trigger as the registration's END condition, not a delayed
// promise (there is no effect-token object to exile): the unscoped
// PlaneswalkedTo fires at any player's planeswalk, so the registered
// CantBeCast restriction carries the UntilPlaneswalk lifetime
// (effEffect's effectPlaneCurseEnd; rules' checkPlaneswalkTriggers ends
// every live one). The kernel lowers the whole shape to ExtPlaneCurse -- the
// targeted seat's bit on the cold planar record (dirac/zone_planecurse.go) that
// planeswalk clears and castForbiddenCost reads -- the narrow equivalent,
// since the record's predicate is target-only: Caster$ Player.IsRemembered
// names the captured player and ValidCard$ Card admits every card.
//
// Duration$ Permanent and RememberObjects$ Targeted are the body's own
// spellings; IsCurse$ True rides along (gorge reads no semantics from it).
// Anything else falls through (done=false) to effectStaticTriggerPair, which
// refuses it by name.
func (lc *lowerCtx) effectPlaneCurse(ps *params, tr string) (bool, error) {
	if !ps.is("Duration", "Permanent") || strings.TrimSpace(ps.str("RememberObjects")) != "Targeted" {
		return false, nil
	}
	if v, ok := ps.m["IsCurse"]; ok && strings.TrimSpace(v) != "True" {
		return true, refuse("Effect PlaneCurse IsCurse$ %q", v)
	}
	statics, _ := ps.get("StaticAbilities")
	sNames := strings.FieldsFunc(statics, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	tNames := strings.FieldsFunc(tr, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	if len(sNames) != 1 || len(tNames) != 1 {
		return false, nil
	}
	ss, err := lc.effectStaticBodies(statics)
	if err != nil {
		return true, err
	}
	if len(ss) != 1 || ss[0].Mode != "CantBeCast" {
		return false, nil
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if strings.TrimSpace(sp.str("ValidCard")) != "Card" || strings.TrimSpace(sp.str("Caster")) != "Player.IsRemembered" {
		return false, nil
	}
	if left := sp.unread(); len(left) > 0 {
		return true, refuse("Effect CantBeCast unread %v", left)
	}
	raw, ok := lc.face.SVars[tNames[0]]
	if !ok {
		return true, refuse("Effect trigger %q", tNames[0])
	}
	t, ok := cards.ParseTriggerLine(raw)
	if !ok {
		return true, refuse("Effect trigger %q", tNames[0])
	}
	if t.Mode != "PlaneswalkedTo" {
		return false, nil
	}
	for k := range t.Params {
		switch k {
		case "Mode", "Execute", "Static", "TriggerDescription":
		default:
			return true, refuse("Effect PlaneswalkedTo trigger %s$", k)
		}
	}
	exec := strings.TrimSpace(t.ParamStr(cards.PKExecute))
	if exec == "" {
		return true, refuse("Effect trigger %q names no Execute", tNames[0])
	}
	body := strings.ReplaceAll(lc.face.SVars[exec], " ", "")
	if !strings.Contains(body, "ChangeZone") || !strings.Contains(body, "Origin$Command") ||
		!strings.Contains(body, "Destination$Exile") || !strings.Contains(body, "Defined$Self") {
		return false, nil
	}
	if strings.TrimSpace(ps.str("ValidTgts")) != "Player" {
		return false, nil
	}
	for _, k := range []string{"TargetMin", "TargetMax", "TgtZone", "TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TargetsWithDefinedController"} {
		if _, ok := ps.m[k]; ok {
			return true, refuse("Effect PlaneCurse %s$", k)
		}
	}
	ps.consume("Duration", "RememberObjects", "IsCurse", "StaticAbilities", "Triggers")
	capture, err := lc.objOrTarget(ps, "", zNone)
	if err != nil {
		return true, err
	}
	if capture.Kind != rTarget {
		return true, refuse("Effect PlaneCurse over %s", capture)
	}
	lc.emit(lInstr{Op: "PlaneCurse", Obj: capture})
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("Effect PlaneCurse unread %v", left)
	}
	return true, nil
}
