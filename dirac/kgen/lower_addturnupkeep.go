package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// addTurnUpkeepRider lowers Alchemist's Gambit's ExtraTurnDelayedTrigger$:
// a ThisTurn$ True | Static$ True | Mode$ Phase | Phase$ Upkeep rider whose
// Execute is a DB$ Effect with one global CantPreventDamage static and, on
// the uncleaved face, one Triggers$ body: Mode$ Phase | Phase$ End of Turn
// executing DB$ LosesGame on You.
//
// gorge registers the rider at the -1 ExtraTurn consumption with the granted
// turn as MinTurn (engine/events/apply_turn.go foldExtraTurn; effAddTurn
// forwards the Upkeep step and the Static$ mark). At that turn's upkeep the
// static body resolves inline: effEffect registers the CantPreventDamage
// restriction with no Affected$ (global, all damage; rules cantPreventDamage)
// until end of turn, and arms EndLose as a delayed end-step trigger whose
// source is the spell. The kernel keeps the grant's rider pending until the
// first upkeep that happens on or after the granted turn (interp's OpAddTurn,
// act_extraturnnoprev.go; a skipped upkeep fires nothing): B=1 marks the
// rider, whose turn then can't prevent damage, and CS ExtraTurnRider (C) is
// its end-step loss, armed at that upkeep. Every other shape refuses by name.
func (lc *lowerCtx) addTurnUpkeepRider(name string, dp *params, exec string, in *lInstr) error {
	if t := dp.str("ThisTurn"); t != "True" {
		return refuse("AddTurn rider %s: ThisTurn$ %q", name, t)
	}
	if s := dp.str("Static"); s != "True" {
		return refuse("AddTurn rider %s: Phase$ \"Upkeep\" Static$ %q", name, s)
	}
	dp.consume("Mode", "Phase", "ThisTurn", "Static", "TriggerDescription")
	if left := dp.unread(); len(left) > 0 {
		return refuse("AddTurn rider %s: unread %v", name, left)
	}
	ex := cards.ResolveSVar(lc.face.SVars, exec)
	if ex == nil {
		return refuse("AddTurn rider Execute$ %q: no such SVar", exec)
	}
	if ex.API != "Effect" {
		return refuse("AddTurn rider Execute$ %s: upkeep api %s", exec, ex.API)
	}
	ep := newParams(ex.Params)
	if d := strings.TrimSpace(ep.str("Defined")); d != "" && d != "You" {
		return refuse("AddTurn rider Execute$ %s: Defined$ %q", exec, d)
	}
	st, _ := ep.get("StaticAbilities")
	if err := lc.addTurnNoPrevStatic(exec, strings.TrimSpace(st)); err != nil {
		return err
	}
	lose := false
	if tr, ok := ep.get("Triggers"); ok {
		if err := lc.addTurnEndLose(exec, strings.TrimSpace(tr)); err != nil {
			return err
		}
		lose = true
	}
	ep.consume("Defined")
	if left := ep.unread(); len(left) > 0 {
		return refuse("AddTurn rider Execute$ %s: unread %v", exec, left)
	}
	in.CS = "ExtraTurnNoPrev" // opAddTurn: B=1, the granted turn's damage can't be prevented
	if lose {
		in.CS = "ExtraTurnRider|ExtraTurnNoPrev" // and C names its end step's loss (kop.StepEnd)
	}
	return nil
}

// addTurnNoPrevStatic accepts the rider Effect's one static: a bare Mode$
// CantPreventDamage (no Affected$, IsCombat$, ValidSource$ or condition),
// gorge's global restriction.
func (lc *lowerCtx) addTurnNoPrevStatic(exec, list string) error {
	if list == "" || strings.ContainsAny(list, ", ") {
		return refuse("AddTurn rider Execute$ %s: StaticAbilities$ %q", exec, list)
	}
	ss, err := lc.effectStaticBodies(list)
	if err != nil {
		return err
	}
	if len(ss) != 1 || ss[0].Mode != "CantPreventDamage" {
		return refuse("AddTurn rider Execute$ %s: static %q not a bare CantPreventDamage", exec, list)
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if left := sp.unread(); len(left) > 0 {
		return refuse("AddTurn rider Execute$ %s: CantPreventDamage unread %v", exec, left)
	}
	return nil
}

// addTurnEndLose accepts the rider Effect's one Triggers$ body: Mode$ Phase |
// Phase$ End of Turn executing DB$ LosesGame on You (Alchemist's Gambit's
// EndLose), the end-step loss the kernel's extra-turn rider fires.
func (lc *lowerCtx) addTurnEndLose(exec, name string) error {
	if name == "" || strings.ContainsAny(name, ", ") {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %q", exec, name)
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %q", exec, name)
	}
	tp := newParams(t.Params)
	if tp.str("Mode") != "Phase" || tp.str("Phase") != "End of Turn" {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %s Mode$ %q Phase$ %q", exec, name, tp.str("Mode"), tp.str("Phase"))
	}
	lx := cards.ResolveSVar(lc.face.SVars, tp.str("Execute"))
	if lx == nil || lx.API != "LosesGame" {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %s Execute$ not DB$ LosesGame", exec, name)
	}
	if d := strings.TrimSpace(newParams(lx.Params).str("Defined")); d != "" && d != "You" {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %s Defined$ %q", exec, name, d)
	}
	tp.consume("Mode", "Phase", "Execute", "TriggerDescription")
	if left := tp.unread(); len(left) > 0 {
		return refuse("AddTurn rider Execute$ %s: Triggers$ %s unread %v", exec, name, left)
	}
	return nil
}
