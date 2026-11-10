package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

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
	grant := tr == nil && ro == "Remembered"
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
	case grant && vc == "Card.nonLand+IsRemembered":
		c |= kop.StackExileGrant
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
