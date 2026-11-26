package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// effectPreventSpell lowers the Effect that stops a targeted spell dealing
// damage while it is on the stack (Dromoka's Command: "Prevent all damage
// target instant or sorcery spell would deal this turn"): DB$ Effect |
// ValidTgts$ <spell filter> | TgtZone$ Stack | RememberObjects$
// TargetedSource | ExileOnMoved$ Stack | StaticAbilities$ <one
// Mode$ Continuous | AffectedZone$ Stack | Affected$ Card.IsRemembered |
// AddKeyword$ Prevent all damage that would be dealt by CARDNAME.>. gorge's
// Effect object grants the keyword to the remembered spell for as long as it
// is on the stack (the spell's own damage is replaced by prevention) and is
// exiled as the spell leaves it. The kernel keeps it as a chosen-source
// shield on the spell at its cast version (kop.PrevNextChosenSpell: every
// event from it, never spent), so a later cast of the same card is not
// covered. It reports false, consuming nothing, for any other Effect.
func (lc *lowerCtx) effectPreventSpell(ps *params) (bool, error) {
	if ps.peek("TgtZone") != "Stack" || ps.peek("RememberObjects") != "TargetedSource" {
		return false, nil
	}
	if ps.peek("ExileOnMoved") != "Stack" || ps.peek("ValidTgts") == "" {
		return true, refuse("Effect RememberObjects %q beside ExileOnMoved$ %q", ps.peek("RememberObjects"), ps.peek("ExileOnMoved"))
	}
	name := ps.peek("StaticAbilities")
	st := etbParams(lc.face.SVars[name])
	if name == "" || strings.Contains(name, ",") || len(st) != 5 || st["Mode"] != "Continuous" || st["AffectedZone"] != "Stack" ||
		st["Affected"] != "Card.IsRemembered" || strings.TrimSpace(st["AddKeyword"]) != "Prevent all damage that would be dealt by CARDNAME." {
		return true, refuse("Effect StaticAbilities$ %q beside RememberObjects$ TargetedSource", name)
	}
	if _, ok := st["Description"]; !ok {
		return true, refuse("Effect StaticAbilities$ %q beside RememberObjects$ TargetedSource", name)
	}
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return true, err
	}
	ps.consume("TgtZone", "RememberObjects", "ExileOnMoved", "StaticAbilities", "TgtPrompt", "AILogic")
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	c := kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextChosen | kop.PrevNextChosenAny | kop.PrevNextChosenTurn | kop.PrevNextChosenSpell
	lc.emit(lInstr{Op: "PreventNext", Obj: lRef{Kind: rTarget, N: t}, C: c, A: cst(0),
		Note: "prevent all damage the targeted spell would deal while it is on the stack"})
	return true, nil
}
