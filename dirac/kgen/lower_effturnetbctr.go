package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectTurnEtbCounter lowers Zameck Guildmage's activated Effect: a turn-long
// Moved replacement over the controller's creatures that makes each of them
// enter with an additional +1/+1 counter (gorge's Effect-created
// PutCounter|ETB$ True body on the replaced card, rules/entry_counters.go
// effectEntryBodies).
//
// The exact shape only:
//
//	DB$ Effect | ReplacementEffects$ Name
//	Event$ Moved | ReplacementResult$ Updated | ActiveZones$ Command |
//	Destination$ Battlefield | ValidCard$ Creature.YouCtrl | ReplaceWith$ Body
//	DB$ PutCounter | ETB$ True | Defined$ ReplacedCard |
//	CounterType$ P1P1 | CounterNum$ 1
//
// No Duration$ (the rider lasts this turn). The replacement's ActiveZones$
// gate is Command, as the carriers print it; it is not consulted after the
// Effect resolves. Combine Guildmage's Creature.YouCtrl+Other line names a
// shape the kernel does not model and refuses by name, and every other body,
// zone or extra key refuses, so effectPrevent's remaining arms and its named
// refusal are unchanged for other shapes. dirac/repl_turnrepl.go applies the
// per-player count at entry (enterGen).
func (lc *lowerCtx) effectTurnEtbCounter(ps *params) (bool, error) {
	if _, ok := ps.m["Duration"]; ok {
		return false, nil // the Duration$ arms own those lifetimes
	}
	name := ps.str("ReplacementEffects")
	if strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Moved" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("ReplacementResult", "Updated") || !rp.is("Destination", "Battlefield") ||
		!rp.is("ActiveZones", "Command") {
		return false, nil
	}
	vc := rp.str("ValidCard")
	if !strings.HasPrefix(vc, "Creature.YouCtrl") {
		return false, nil
	}
	if vc != "Creature.YouCtrl" {
		// Combine Guildmage's +Other line: a shape this arm reads but does
		// not model, named here instead of the generic refusal.
		return false, refuse("Effect ETB-counter turn replacement ValidCard$ %q", vc)
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if body == nil || len(body) != 5 || body["DB"] != "PutCounter" || body["ETB"] != "True" ||
		body["Defined"] != "ReplacedCard" || body["CounterType"] != "P1P1" || body["CounterNum"] != "1" {
		return false, refuse("Effect ETB-counter turn replacement body")
	}
	if left := rp.unread(); len(left) > 0 {
		return false, refuse("Effect ETB-counter turn replacement unread %v", left)
	}
	if left := ps.unread(); len(left) > 0 {
		return false, refuse("Effect ETB-counter turn replacement unread %v", left)
	}
	lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplEtbCreatureCtr, Note: "each creature the controller controls enters with an additional +1/+1 counter until end of turn"})
	return true, nil
}
