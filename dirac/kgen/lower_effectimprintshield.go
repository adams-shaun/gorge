package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_effectimprintshield.go: the source-keyed "prevent the next damage"
// shield an Effect keeps on its own Imprinted list (Dazzling Reflection):
//
//	DB$ Effect | ReplacementEffects$ R | ImprintCards$ Targeted | Triggers$ T
//	R: Event$ DamageDone | ValidSource$ Card.IsImprinted | PreventionEffect$ True | ReplaceWith$ <self-exile>
//	T: Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ <Creature|Card>.IsImprinted | Execute$ <self-exile> | Static$ True
//
// gorge's effEffect imprints the parent's target on the created Effect, the
// replacement prevents the next damage that creature deals and exiles the
// Effect, and the static leave trigger exiles the Effect when the creature
// leaves the battlefield first. That is Awe Strike's shield without its gain
// rider (lower_preventrider.go): RememberObjects$ Targeted plus ExileOnMoved$
// Battlefield, the effect's own list read by its own lines only. The kernel's
// record is the same Ext PreventNext, C PrevNextAll|PrevNextFrom, keyed on
// the target creature: spent by the first event it prevents, ended when the
// creature leaves first (repl_prevnext.go). Any other line, rider or extra
// parameter keeps the effect on its old route (and its refusal).
func (lc *lowerCtx) effectImprintShieldFrom(ps *params) (bool, error) {
	if strings.TrimSpace(ps.peek("ImprintCards")) != "Targeted" {
		return false, nil
	}
	for _, k := range []string{"ValidTgts", "RememberObjects", "Duration", "StaticAbilities", "ExileOnMoved", "ForgetOnMoved", "Defined"} {
		if _, has := ps.m[k]; has {
			return false, nil
		}
	}
	rname := strings.TrimSpace(ps.peek("ReplacementEffects"))
	tname := strings.TrimSpace(ps.peek("Triggers"))
	if rname == "" || tname == "" || strings.Contains(rname, ",") || strings.Contains(tname, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[rname])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rm := r.Params
	if strings.TrimSpace(rm["PreventionEffect"]) != "True" || strings.TrimSpace(rm["ValidSource"]) != "Card.IsImprinted" ||
		!lc.bodyIsSelfExile(strings.TrimSpace(rm["ReplaceWith"])) {
		return false, nil
	}
	rp := newParams(rm)
	rp.consume("Event", "PreventionEffect", "ValidSource", "ReplaceWith", "Description")
	if len(rp.unread()) > 0 {
		return false, nil
	}
	if !lc.imprintLeaveExileTrigger(tname) {
		return false, nil
	}
	obj, err := lc.defined("Targeted")
	if err != nil {
		return true, err
	}
	ps.consume("ImprintCards", "ReplacementEffects", "Triggers", "AILogic", "Name")
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: kop.PrevNextAll | kop.PrevNextFrom,
		Note: "the next damage the imprinted creature deals is prevented; the effect ends when it leaves"})
	return true, nil
}

// imprintLeaveExileTrigger reports whether SVar name is exactly the Effect's
// static leave trigger on its imprinted creature that exiles the Effect:
// gorge's ExileOnMoved$ Battlefield, spelled as a trigger.
func (lc *lowerCtx) imprintLeaveExileTrigger(name string) bool {
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok || t.Mode != "ChangesZone" {
		return false
	}
	tp := newParams(t.Params)
	vc := strings.TrimSpace(tp.peek("ValidCard"))
	if strings.TrimSpace(tp.peek("Origin")) != "Battlefield" || strings.TrimSpace(tp.peek("Destination")) != "Any" ||
		vc != "Creature.IsImprinted" && vc != "Card.IsImprinted" ||
		strings.TrimSpace(tp.peek("Static")) != "True" || !lc.bodyIsSelfExile(strings.TrimSpace(tp.peek("Execute"))) {
		return false
	}
	tp.consume("Mode", "Origin", "Destination", "ValidCard", "Static", "Execute", "TriggerDescription", "Description")
	return len(tp.unread()) == 0
}
