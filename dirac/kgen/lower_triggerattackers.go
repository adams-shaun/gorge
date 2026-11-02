package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Defined$ TriggeredAttackers, gorge's plural
// attack-batch referent (definedSpecTriggeredAttackers reads the queue
// entry's Remembered: the attackers of the DeclareAttackers event that
// raised the trigger, not the subset the line's ValidAttackers$ matched).
//
// The kernel captures that group in the pending trigger and runs exactly
// three ops over it, one attacker at a time (OpPumpUntilEOT, OpPutCounter,
// OpGrantKeyword; kernel execTriggerAttackers). Any other op reading the
// ref would resolve no object and silently do nothing, so the ref is
// gated twice: triggerAttackersSubjectOK at the Defined$ read refuses the
// shapes whose lowering this file does not vouch for, and
// checkTriggerAttackers (called by chain after each node) refuses any
// emitted instruction that holds the ref beside an op outside the three.

// triggerAttackersOps are the ops the kernel runs over the group.
var triggerAttackersOps = map[string]bool{"PumpUntilEOT": true, "PutCounter": true, "GrantKeyword": true}

// triggerAttackersSubjectOK admits Defined$ TriggeredAttackers only as the
// subject of a plain Pump or PutCounter node of an AttackersDeclared
// trigger.
func (lc *lowerCtx) triggerAttackersSubjectOK() error {
	if lc.ab == nil || lc.ab.Trigger == nil || lc.ab.Trigger.Event != "AttackersDeclared" {
		return refuse("Defined$ \"TriggeredAttackers\" outside an AttackersDeclared trigger")
	}
	n := lc.curNode
	if n == nil || lc.condDefined || n.ParamStr(cards.PKDefined) != "TriggeredAttackers" {
		return refuse("Defined$ \"TriggeredAttackers\" read by something other than the node's subject")
	}
	has := func(k string) bool { _, ok := n.Params[k]; return ok }
	switch n.API {
	case "Pump":
		for _, k := range []string{"Duration", "PumpZone", "AtEOT", "KWChoice", "ReplaceDyingDefined", "ValidTgts"} {
			if has(k) {
				return refuse("Defined$ \"TriggeredAttackers\" on Pump with %s$", k)
			}
		}
		for _, k := range []string{"NumAtt", "NumDef"} {
			if strings.TrimSpace(n.Params[k]) == "Double" {
				return refuse("Defined$ \"TriggeredAttackers\" on Pump with %s$ Double", k)
			}
		}
		if kw := n.Params["KW"]; strings.Contains(kw, "must be blocked") || strings.Contains(kw, "Firebending") ||
			strings.Contains(kw, skipNextUntapKW) {
			return refuse("Defined$ \"TriggeredAttackers\" on Pump with a KW$ rider")
		}
	case "PutCounter":
		for _, k := range []string{"DividedAsYouChoose", "ETB", "Bolster", "Support", "Choices", "Adapt", "Monstrosity",
			"CounterTypes", "EachFromSource", "RememberPut", "RememberAmount", "ValidTgts"} {
			if has(k) {
				return refuse("Defined$ \"TriggeredAttackers\" on PutCounter with %s$", k)
			}
		}
		if ct := strings.TrimSpace(n.Params["CounterType"]); ct == "" || ct == "ENERGY" {
			return refuse("Defined$ \"TriggeredAttackers\" on PutCounter with CounterType$ %q", ct)
		}
	default:
		return refuse("Defined$ \"TriggeredAttackers\" on %s", n.API)
	}
	return nil
}

// checkTriggerAttackers refuses a node whose instructions (from index at)
// hold the TriggeredAttackers group in an op the kernel does not run over
// it, or inside a value (a Double read of the group's power).
func (lc *lowerCtx) checkTriggerAttackers(at int) error {
	if at > len(lc.ab.Prog) {
		return nil
	}
	for _, in := range lc.ab.Prog[at:] {
		inVal := in.A.Ref.Kind == rTriggerAttackers || in.B.Ref.Kind == rTriggerAttackers
		if inVal || (in.Obj.Kind == rTriggerAttackers || in.Obj2.Kind == rTriggerAttackers) && !triggerAttackersOps[in.Op] {
			return refuse("Defined$ \"TriggeredAttackers\" under %s", in.Op)
		}
	}
	return nil
}
