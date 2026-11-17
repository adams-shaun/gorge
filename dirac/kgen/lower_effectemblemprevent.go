package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectDurationPrevent is effectPrevent's Duration$ arm past the lifetimes it
// lowers itself: Ajani Steadfast's Permanent emblem (effectEmblemPrevent),
// and the Effect replacement Duration$ refusal for every other shape. It keeps
// effectPrevent under codeshape's complexity threshold.
func (lc *lowerCtx) effectDurationPrevent(ps *params) error {
	if ok, err := lc.effectEmblemPrevent(ps); ok || err != nil {
		return err
	}
	return refuse("Effect replacement Duration$")
}

// effectEmblemPrevent lowers a DB$ Effect with Duration$ Permanent whose one
// ReplacementEffects$ line is a static all-but-N damage prevention (Ajani
// Steadfast's -7: Event$ DamageDone | ValidTarget$ You,Planeswalker.YouCtrl |
// ReplaceWith$ a DB$ ReplaceDamage | Amount$ ReplaceCount$DamageAmount/Minus.N
// body): gorge registers it as an effect-created replacement for the rest of
// the game, the kernel's emblem with none of the lifetime flags. The line is
// read by the static reader (staticDamagePrevent: the same recipient, source,
// gate and amount reads a printed static prevention has) and rides
// lEmblem.Repls as a PreventDamageN replacement (dirac/repl_staticprevent_emblem.go).
// An emblem has no host: a body that reads one (a counter or value amount, an
// Optional$ line, a gate, a Self or chosen filter) refuses. It reports false,
// consuming nothing, for any other shape so effectDurationPrevent keeps its
// Duration$ refusal (Dread Wight).
func (lc *lowerCtx) effectEmblemPrevent(ps *params) (bool, error) {
	if !ps.is("Duration", "Permanent") {
		return false, nil
	}
	if ps.peek("Stackable") != "False" {
		return false, nil // the kernel makes the emblem once (permEmblemHeld)
	}
	for _, k := range []string{"RememberObjects", "ValidTgts", "Triggers", "StaticAbilities", "ExileOnMoved", "ForgetOnMoved"} {
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
	w := cards.ResolveSVar(lc.face.SVars, r.Params["ReplaceWith"])
	if w == nil || w.API != "ReplaceDamage" {
		return false, nil
	}
	// gorge registers an Effect's line in the Command zone; the static reader
	// reads the host-bound spelling, which it names ActiveZones$ Battlefield
	if az, has := r.Params["ActiveZones"]; has && strings.TrimSpace(az) != "Command" {
		return false, nil
	}
	cp := make(map[string]string, len(r.Params)+1)
	for k, v := range r.Params {
		cp[k] = v
	}
	cp["ActiveZones"] = "Battlefield"
	rp := newParams(cp)
	rp.consume("Event", "ReplaceWith", "ReplacementResult", "Description")
	var rl lRepl
	if ok, err := lc.staticDamagePrevent(rp, w, &rl); !ok || err != nil { // lower_staticprevent.go
		return ok, err
	}
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("Effect replacement unread %v", left)
	}
	const hostRead = kop.ReplSPOptional | kop.ReplSPLimitCtr | kop.ReplSPAmtV | kop.ReplSPDmgLE
	if rl.Kind != "PreventDamageN" || rl.Bits&kop.ReplSPAllBut == 0 || rl.Bits&hostRead != 0 || rl.Cond.Kind != cNone {
		return true, refuse("Effect replacement Duration$")
	}
	if err := emblemFiltersRefuse("Effect replacement", rl.F, rl.F2); err != nil {
		return true, err
	}
	if _, _, _, err := effectEmblemDurationAs(ps, true); err != nil {
		return true, err
	}
	ps.consume("Duration", "ReplacementEffects", "Name", "Image", "Stackable", "AILogic")
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("Effect replacement unread %v", left)
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Repls: []lRepl{rl}})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1, Note: "the controller gets an emblem that prevents all but N of the damage to them and their planeswalkers"})
	return true, nil
}
