package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// effectStackExile lowers the Effect that exiles a named spell instead of
// putting it into a graveyard as it leaves the stack (gorge effects/effect.go
// registers a live Event$ Moved | Origin$ Stack | Destination$ Graveyard
// replacement over the remembered card; ExileOnMoved$ Stack ends it as the
// spell leaves the stack). Two shapes, one OpExt ExtStackExileMark:
//
//   - a SpellCast trigger's DB$ Effect | RememberObjects$ TriggeredCard |
//     ExileOnMoved$ Stack | ConditionDefined$ TriggeredCard | ConditionPresent$
//     Card + ConditionZone$ Stack (or Card.inZoneStack) whose replacement has
//     ValidCard$ Card.IsRemembered+YouOwn and a ChangeZone body of the
//     ReplacedCard to exile: plain, with RememberChanged$ + an AlterAttribute
//     Plotted rider (Lilah, Undefeated Slickshot), or with WithCountersType$
//     DREAM (Goliath Daydreamer: the mark stalls, the kernel holds no such
//     counter).
//   - an Effect after the exile of a card with RememberObjects$ Remembered |
//     ExileOnMoved$ Stack whose replacement has ValidCard$ Card.nonLand+
//     IsRemembered and the plain body (Nita, Forum Conciliator): the card's
//     may-play permission carries the mark into the cast.
//
// It reports false for any other shape so effectPrevent's refusals stand; a
// shape it reads but does not recognise refuses by name.
func (lc *lowerCtx) effectStackExile(ps *params) (bool, error) {
	if ps.m["ExileOnMoved"] != "Stack" || ps.m["ReplacementEffects"] == "" {
		return false, nil
	}
	if _, ok := ps.m["StaticAbilities"]; ok {
		return false, nil
	}
	if _, ok := ps.m["Duration"]; ok {
		return false, nil
	}
	ro := ps.m["RememberObjects"]
	tr := lc.ab.Trigger
	trig := tr != nil && tr.Event == "SpellCast" && lc.trigBody && !lc.ab.Latent && ro == "TriggeredCard"
	// the grant form: the remembered exiled card (Nita, or Dire Fleet
	// Daredevil's enters trigger) or the targeted graveyard card (Sins of the
	// Past, Jace, Telepath Unbound) whose may-play permission the same
	// ability granted just before
	grant := !trig && (ro == "Remembered" || ro == "Targeted" && lc.mayPlayOn(rTarget))
	if !trig && !grant {
		return false, nil
	}
	if _, ok := ps.m["SetChosenNumber"]; ok {
		return false, nil
	}
	name := ps.str("ReplacementEffects")
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "Moved" || r["Destination"] != "Graveyard" {
		return false, nil
	}
	rp := newParams(r)
	rp.consume("Event", "Description")
	if !rp.is("Origin", "Stack") || !rp.is("Destination", "Graveyard") {
		return false, refuse("Effect stack-exile replacement Origin$/Destination$")
	}
	var c int
	if f, ok := rp.get("Fizzle"); ok {
		if f != "False" {
			return false, refuse("Effect stack-exile replacement Fizzle$ %q", f)
		}
		c |= kop.StackExileResolved // gorge matches the resolution move alone (stackLeaveByFizzle)
	}
	vc := rp.str("ValidCard")
	switch {
	case trig && vc == "Card.IsRemembered+YouOwn":
		c |= kop.StackExileOwn
	case grant && (vc == "Card.nonLand+IsRemembered" || vc == "Card.IsRemembered"):
		// Origin$ Stack: a land is never on the stack, so the nonLand word
		// matches the same moves
		c |= kop.StackExileGrant
		if ro == "Targeted" {
			c |= kop.StackExileTarget
		}
	default:
		return false, refuse("Effect stack-exile replacement ValidCard$ %q", vc)
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if left := rp.unread(); len(left) > 0 {
		return false, refuse("Effect stack-exile replacement unread %v", left)
	}
	if body == nil || body["DB"] != "ChangeZone" || body["Defined"] != "ReplacedCard" ||
		body["Origin"] != "Stack" || body["Destination"] != "Exile" {
		return false, refuse("Effect stack-exile replacement body")
	}
	switch len(body) {
	case 4:
		// the plain exile
	case 5:
		if body["WithCountersType"] != "DREAM" || !trig {
			return false, refuse("Effect stack-exile replacement body")
		}
		c |= kop.StackExileDream
	case 6:
		if body["RememberChanged"] != "True" || !trig || !lc.stackExilePlotSub(body["SubAbility"]) {
			return false, refuse("Effect stack-exile replacement body")
		}
		c |= kop.StackExilePlot
	default:
		return false, refuse("Effect stack-exile replacement body")
	}
	if trig {
		// the spell must still be on the stack when the Effect resolves
		cd, pr, zn := ps.m["ConditionDefined"], ps.m["ConditionPresent"], ps.m["ConditionZone"]
		if cd != "TriggeredCard" || !(pr == "Card" && zn == "Stack" || pr == "Card.inZoneStack" && zn == "") {
			return false, refuse("Effect stack-exile condition")
		}
		ps.consume("ConditionDefined", "ConditionPresent", "ConditionZone")
	}
	ps.consume("ExileOnMoved", "RememberObjects", "ReplacementEffects")
	obj, err := lc.defined(ro)
	if err != nil {
		return false, err
	}
	lc.emit(lInstr{Op: "StackExileMark", Obj: obj, C: c})
	return true, nil
}

// stackExileChosenMark lowers Mission Briefing's chosen-card stack-exit
// replacement: the Effect grants the chosen card's may-play permission and
// its replacement exiles that card instead of putting it into a graveyard as
// it leaves the stack. The permission is already emitted, so the mark names
// the chosen reference (StackExileTarget) and stackExileGrantOn marks its
// record. Only the exact replacement body lowers.
func (lc *lowerCtx) stackExileChosenMark(ps *params, obj lRef) error {
	if obj.Kind != rChosen {
		return nil
	}
	if _, ok := ps.m["ReplacementEffects"]; !ok {
		return nil
	}
	if strings.TrimSpace(ps.m["ExileOnMoved"]) != "Stack" {
		return refuse("Effect chosen-card stack-exile ExileOnMoved$ %q", ps.m["ExileOnMoved"])
	}
	name := ps.str("ReplacementEffects")
	if name == "" || strings.Contains(name, ",") {
		return refuse("Effect chosen-card stack-exile ReplacementEffects$ %q", name)
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil {
		return refuse("Effect chosen-card stack-exile ReplacementEffects$ %q", name)
	}
	rp := newParams(r)
	rp.consume("Event", "Description")
	if !rp.is("Origin", "Stack") || !rp.is("Destination", "Graveyard") {
		return refuse("Effect chosen-card stack-exile Origin$/Destination$")
	}
	switch vc := rp.str("ValidCard"); vc {
	case "Card.IsRemembered", "Card.nonLand+IsRemembered":
	default:
		return refuse("Effect chosen-card stack-exile ValidCard$ %q", vc)
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if left := rp.unread(); len(left) > 0 {
		return refuse("Effect chosen-card stack-exile replacement unread %v", left)
	}
	if body == nil || body["DB"] != "ChangeZone" || body["Defined"] != "ReplacedCard" ||
		body["Origin"] != "Stack" || body["Destination"] != "Exile" || len(body) != 4 {
		return refuse("Effect chosen-card stack-exile replacement body")
	}
	ps.consume("ExileOnMoved", "ReplacementEffects")
	lc.emit(lInstr{Op: "StackExileMark", Obj: obj, C: kop.StackExileGrant | kop.StackExileTarget})
	return nil
}

// stackExilePlotSub verifies a replacement body's SubAbility is the exact
// plot rider Lilah spells: AlterAttribute Attributes$ Plotted over
// Defined$ Remembered, then Cleanup ClearRemembered$ True.
func (lc *lowerCtx) stackExilePlotSub(name string) bool {
	p := etbParams(lc.face.SVars[name])
	if p == nil || p["DB"] != "AlterAttribute" || p["Defined"] != "Remembered" || p["Attributes"] != "Plotted" {
		return false
	}
	for k := range p {
		switch k {
		case "DB", "Defined", "Attributes", "StackDescription", "SpellDescription", "SubAbility":
		default:
			return false
		}
	}
	cl := etbParams(lc.face.SVars[p["SubAbility"]])
	return cl != nil && cl["DB"] == "Cleanup" && cl["ClearRemembered"] == "True" && len(cl) == 2
}

// mayPlayOn reports whether the ability's program so far grants a may-play
// permission on a reference of kind kind: the grant form marks that record,
// so a permission must exist for the mark to ride.
func (lc *lowerCtx) mayPlayOn(kind refKind) bool {
	for _, in := range lc.ab.Prog {
		if in.Op == "MayPlay" && in.Obj.Kind == kind {
			return true
		}
	}
	return false
}
