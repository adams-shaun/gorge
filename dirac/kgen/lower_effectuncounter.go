package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectUncounterScope lowers a DB$ Effect whose one bodyless Counter
// replacement stops the Counter event over every spell the Effect's
// controller controls this turn (gorge counterReplacementMatchesAll's
// effect-created arm, no remembered card, UntilEOT because no Duration$ is
// given): Veil of Summer's "Spells you control can't be countered this turn"
// and Domri, Anarch of Bolas's "Creature spells you cast this turn can't be
// countered":
//
//	DB$ Effect | ReplacementEffects$ Name
//	Event$ Counter | ValidSA$ Spell.YouCtrl | Layer$ CantHappen
//	Event$ Counter | ValidSA$ Spell.Creature+wasCastByYou | Layer$ CantHappen
//
// One OpExt TurnRepl TurnReplUncounter or TurnReplUncounterCreature. It holds
// for a spell cast after the Effect resolved too (dirac/repl_turnrepl.go).
// Every other shape reports false, so effectPrevent keeps its named refusal.
func (lc *lowerCtx) effectUncounterScope(ps *params) (bool, error) {
	if _, ok := ps.get("RememberObjects"); ok {
		return false, nil
	}
	name := ps.str("ReplacementEffects")
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Counter" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event", "Description")
	if !rp.is("Layer", "CantHappen") {
		return false, nil
	}
	var kind int
	switch rp.str("ValidSA") {
	case "Spell.YouCtrl":
		kind = kop.TurnReplUncounter
	case "Spell.Creature+wasCastByYou":
		kind = kop.TurnReplUncounterCreature
	default:
		return false, nil
	}
	rp.consume("Layer", "ValidSA")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	if left := ps.unread(); len(left) > 0 {
		return false, nil
	}
	lc.emit(lInstr{Op: "TurnRepl", C: kind, Note: "spells its controller controls (creature spells) can't be countered this turn"})
	return true, nil
}
