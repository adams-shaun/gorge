package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectExileScope lowers Malicious Eclipse's DB$ Effect: one bodyless Moved
// replacement, "if a creature an opponent controls would die this turn, exile
// it instead", whose ValidLKI$ is any creature the Effect's controller's
// opponents control, with no remembered object, ActiveZones$ or Duration$:
//
//	DB$ Effect | ReplacementEffects$ Name
//	Event$ Moved | ValidLKI$ Creature.OppCtrl | Origin$ Battlefield |
//	Destination$ Graveyard | ReplaceWith$ Body
//	DB$ ChangeZone | Defined$ ReplacedCard | Origin$ Battlefield | Destination$ Exile
//
// One OpExt TurnRepl TurnReplExileOpp. It holds for the turn over every such
// creature, one that enters after the Effect resolved included, which the
// per-object ExileExMark cannot (dirac/repl_turnrepl.go). Every other shape
// reports false, so effectPrevent keeps its named refusal.
func (lc *lowerCtx) effectExileScope(ps *params) (bool, error) {
	name := ps.str("ReplacementEffects")
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Moved" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if rp.str("ValidLKI") != "Creature.OppCtrl" || rp.str("Origin") != "Battlefield" || rp.str("Destination") != "Graveyard" {
		return false, nil
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if len(body) != 4 || body["DB"] != "ChangeZone" || body["Defined"] != "ReplacedCard" ||
		body["Origin"] != "Battlefield" || body["Destination"] != "Exile" {
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	if left := ps.unread(); len(left) > 0 {
		return false, nil
	}
	lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplExileOpp, Note: "creatures an opponent controls that would die this turn are exiled instead"})
	return true, nil
}
