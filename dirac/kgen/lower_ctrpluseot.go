package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectCounterPlusEOT lowers Brad Boimler, Eager Ensign's tap-triggered
// Effect: one AddCounter replacement on the resolving controller's permanents
// that puts that many plus one of each kind instead, until end of turn
// (gorge effEffect's bodyless AddCounter replacement with no
// ValidCounterType$, so every kind matches; rules/replacement_match.go).
//
// The exact shape only:
//
//	DB$ Effect | ReplacementEffects$ Name
//	Event$ AddCounter | ActiveZones$ Battlefield |
//	ValidCard$ Permanent.YouCtrl+inZoneBattlefield | ReplaceWith$ Body
//	DB$ ReplaceCounter | ChooseCounter$ True | Amount$ X
//	X: ReplaceCount$CounterNum/Plus.1
//
// Prairie Dog's activated Effect is the same shape over creatures with
// ValidCounterType$ P1P1 in the replacement and its body (and the body's
// Amount$ Y): TurnRepl TurnReplPlusCreatureCtr, a count apart from Boimler's
// (dirac/repl_turnrepl.go).
//
// Every other Effect+AddCounter shape returns false, so effectPrevent's
// existing arms keep their named refusal. The effect's ActiveZones$ gate is
// not consulted after it resolves (gorge's Effect-created replacement ignores
// it), so the kernel flag lasts to cleanup even if Boimler leaves.
func (lc *lowerCtx) effectCounterPlusEOT(ps *params) (bool, error) {
	name := ps.str("ReplacementEffects")
	if strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "AddCounter" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if az, ok := rp.get("ActiveZones"); ok && az != "Battlefield" {
		return false, nil
	}
	// Boimler's shape (every kind on any permanent) or Prairie Dog's (+1/+1 only,
	// on a creature): the second names the kind in the replacement and again in
	// the body, which a stray kind in one but not the other would not match.
	var (
		wantBody = 3
		kind     string
		turnC    int
	)
	switch rp.str("ValidCard") {
	case "Permanent.YouCtrl+inZoneBattlefield":
	case "Creature.YouCtrl+inZoneBattlefield":
		if kind = rp.str("ValidCounterType"); kind != "P1P1" {
			return false, nil
		}
		wantBody, turnC = 4, kop.TurnReplPlusCreatureCtr
	default:
		return false, nil
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if len(body) != wantBody || body["DB"] != "ReplaceCounter" ||
		body["ChooseCounter"] != "True" || body["ValidCounterType"] != kind || turnC == 0 && body["Amount"] != "X" {
		return false, nil
	}
	if lc.face.SVars[body["Amount"]] != "ReplaceCount$CounterNum/Plus.1" {
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	if left := ps.unread(); len(left) > 0 {
		return false, nil
	}
	if turnC != 0 {
		lc.emit(lInstr{Op: "TurnRepl", C: turnC, Note: "one more +1/+1 counter on the controller's creatures until end of turn"})
		return true, nil
	}
	lc.emit(lInstr{Op: "CounterPlusEOT", Note: "counters plus one on the controller's permanents until end of turn"})
	return true, nil
}
