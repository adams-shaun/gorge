package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPrevent lowers a DB$ Effect whose ReplacementEffects$ are gorge's
// bodyless combat-damage prevention lines on the remembered creature
// (effects/effect.go's Prevent$ True DamageDone registration: a this-turn
// grant when no Duration$ is given): Prevent$ True | IsCombat$ True |
// ValidTarget$ (prevented to it) or ValidSource$ (prevented from it)
// Card.IsRemembered. Lowered as an until-end-of-turn keyword grant on the
// remembered object (KwPreventCombatDamageTo / KwPreventCombatDamageFrom):
// the grant ends when the object leaves, as ExileOnMoved$/ForgetOnMoved$
// Battlefield end gorge's. Every other replacement shape refuses.
func (lc *lowerCtx) effectPrevent(ps *params) error {
	if ok, err := lc.effectEtbCounters(ps); ok || err != nil { // lower_effetbctr.go
		return err
	}
	if ok, err := lc.effectStackExile(ps); ok || err != nil { // lower_effectstackexile.go
		return err
	}
	if ok, err := lc.effectCounterPlusEOT(ps); ok || err != nil { // lower_ctrpluseot.go
		return err
	}
	if ok, err := lc.effectExileScope(ps); ok || err != nil { // lower_effectexilescope.go
		return err
	}
	if ok, err := lc.effectUncounterScope(ps); ok || err != nil { // lower_effectuncounter.go
		return err
	}
	if ok, err := lc.effectDurationRoutes(ps); ok || err != nil { // the Duration$ lifetimes
		return err
	}
	if ok, err := lc.effectReplEmblem(ps); ok || err != nil { // lower_effectreplemblem.go
		return err
	}
	if ok, err := lc.effectPreventScoped(ps); ok || err != nil {
		return err
	}
	if ok, err := lc.effectDmgRedirectScope(ps); ok || err != nil { // lower_effectredirect.go
		return err
	}
	if ok, err := lc.effectDmgDoubleBySource(ps); ok || err != nil { // lower_effectdmgmark.go
		return err
	}
	if ok, err := lc.effectPreventAllTo(ps); ok || err != nil { // lower_preventallto.go
		return err
	}
	for _, k := range []string{"ExileOnMoved", "ForgetOnMoved"} {
		if v, ok := ps.get(k); ok && strings.TrimSpace(v) != "Battlefield" {
			return refuse("Effect replacement %s$ %q", k, v)
		}
	}
	var kws []string
	for _, name := range strings.Split(ps.str("ReplacementEffects"), ",") {
		r, ok := cards.ParseReplacementLine(lc.face.SVars[strings.TrimSpace(name)])
		if !ok || r.Event != "DamageDone" {
			return refuse("Effect replacement %q", strings.TrimSpace(name))
		}
		rp := newParams(r.Params)
		rp.consume("Event", "ActiveZones")
		if !rp.is("Prevent", "True") || !rp.is("IsCombat", "True") {
			return refuse("Effect replacement shape")
		}
		vt, vs := rp.str("ValidTarget"), rp.str("ValidSource")
		switch {
		case vt == "Card.IsRemembered" && vs == "":
			kws = append(kws, "PreventCombatDamageTo")
		case vs == "Card.IsRemembered" && vt == "":
			kws = append(kws, "PreventCombatDamageFrom")
		default:
			return refuse("Effect replacement shape")
		}
		if left := rp.unread(); len(left) > 0 {
			return refuse("Effect replacement unread %v", left)
		}
	}
	ro := ps.str("RememberObjects")
	var obj lRef
	var err error
	if _, ok := ps.get("ValidTgts"); ok {
		if ro != "Targeted" && ro != "ThisTargetedCard" {
			return refuse("Effect RememberObjects %q", ro)
		}
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	} else {
		if ro != "Targeted" && ro != "Self" {
			return refuse("Effect RememberObjects %q", ro)
		}
		if obj, err = lc.defined(ro); err != nil {
			return err
		}
	}
	// A = 1: the grant is gorge's Effect object, which names itself in the
	// CR 616.1 order option (the kernel records it: recordPrevGrant). A Pump
	// KW$ grant (Fleeting Flight) is the creature's own keyword, so gorge's
	// option names the creature, and A stays 0.
	lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, A: cst(1), CS: strings.Join(kws, "&")})
	return nil
}

// effectDurationRoutes is the Duration$ dispatch that runs before the
// no-Duration arms: the lifetimes effectPrevent lowers itself (claimed: the
// arm that takes the line terminates the node, as its inline form did), then
// every other Duration$ to effectPreventDuration. It reports false,
// consuming nothing, when no Duration$ names a route.
func (lc *lowerCtx) effectDurationRoutes(ps *params) (bool, error) {
	if ps.is("Duration", "UntilTheEndOfYourNextUntap") {
		return true, lc.skipUntapEffect(ps)
	}
	if ps.is("Duration", "AsLongAsControl") || ps.is("Duration", "AsLongAsInPlay") || ps.is("Duration", "UntilHostLeavesPlay") || ps.is("Duration", "UntilUntaps") {
		if ok, err := lc.untapLockEffect(ps); ok || err != nil { // lower_untaplock.go
			return true, err
		}
	}
	if ps.is("Duration", "UntilYourNextTurn") {
		return true, lc.effectDmgMark(ps) // lower_effectdmgmark.go
	}
	if ps.is("Duration", "UntilHostLeavesPlayOrEOT") {
		return true, lc.effectDmgRedirectNext(ps) // lower_effectredirect.go
	}
	if _, ok := ps.get("Duration"); ok {
		return true, lc.effectPreventDuration(ps)
	}
	return false, nil
}

// effectPreventDuration lowers a prevention Effect that carries a Duration$.
// Duration$ UntilEndOfCombat is the scoped prevention that ends with the combat
// (Suppressor Skyguard); an out-of-shape line keeps the Effect replacement
// Duration$ refusal (Ria Ivor's remembered source is prevstate-04). Every other
// Duration$ goes to lower_effectemblemprevent.go (Ajani's Permanent emblem,
// else refuses).
func (lc *lowerCtx) effectPreventDuration(ps *params) error {
	if ps.is("Duration", "UntilEndOfCombat") {
		if ok, err := lc.effectPreventScoped(ps); ok || err != nil {
			return err
		}
	}
	return lc.effectDurationPrevent(ps)
}

// effectPreventScoped tries the three this-turn scoped-prevention arms in
// turn: the bodyless Prevent$ True line (effectPreventScope), the gain-rider
// form (effectPreventRider, lower_preventrider.go) and Awe Strike's
// remembered-source form (effectPreventRiderFrom, also
// lower_preventrider.go). A Duration$ UntilEndOfCombat on the Effect ends the
// record with the combat (kop.PrevScopeEndCombat) instead of the turn. It
// keeps effectPrevent under the complexity ceiling.
func (lc *lowerCtx) effectPreventScoped(ps *params) (bool, error) {
	eoc := ps.is("Duration", "UntilEndOfCombat")
	if ok, err := lc.effectPreventScope(ps, eoc); ok || err != nil {
		return ok, err
	}
	if ok, err := lc.effectPreventRider(ps, eoc); ok || err != nil {
		return ok, err
	}
	return lc.effectPreventRiderFrom(ps)
}

// effectPreventScope lowers the this-turn prevention Effect that names no
// remembered object: one bodyless Event$ DamageDone | Prevent$ True line in
// the Command zone (or ActiveZones$ Battlefield, which gorge does not read for
// an Effect-created line) whose ValidTarget$ is You, Creature.YouCtrl, both,
// or none (any recipient: Ethereal Haze), with ValidSource$ Creature.
// inZoneBattlefield, a Card.<Colour> list (Surge of Salvation) or none, and
// IsCombat$ True (combat damage only: Loyal Unicorn) or none (combat and
// noncombat alike). eoc (Duration$ UntilEndOfCombat: Suppressor Skyguard)
// marks the record kop.PrevScopeEndCombat. One OpExt PreventDamageScope. It
// reports false for any other shape so effectPrevent's own arms read it.
func (lc *lowerCtx) effectPreventScope(ps *params, eoc bool) (bool, error) {
	if _, ok := ps.get("RememberObjects"); ok {
		return false, nil
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return false, nil
	}
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	// an absent ActiveZones$ is the Command zone: gorge registers an Effect's
	// line there either way (Take the Bait)
	if az, has := rp.get("ActiveZones"); !rp.is("Prevent", "True") || has && strings.TrimSpace(az) != "Command" && strings.TrimSpace(az) != "Battlefield" {
		return false, nil
	}
	bits := 0
	if v, ok := rp.get("IsCombat"); ok {
		if strings.TrimSpace(v) != "True" {
			return false, nil
		}
		bits |= kop.PrevScopeCombat
	}
	tb, tok := scopeTargetBits(rp.str("ValidTarget"))
	sb, sok := scopeSourceBits(rp.str("ValidSource"))
	var sf, tf *lFilter
	if tok && sok {
		bits |= tb | sb
	} else {
		fb, f1, f2, err := lc.scopeFiltered(rp.str("ValidTarget"), rp.str("ValidSource"), tb, tok, sb, sok) // lower_prevscopefilter.go
		if err != nil {
			return true, err
		}
		bits |= fb
		sf, tf = f1, f2
	}
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("Effect replacement unread %v", left)
	}
	if v, ok := ps.get("ExileOnLost"); ok && strings.TrimSpace(v) == "True" {
		// Surge of Salvation: it ends the Effect when its remembered card is
		// lost, and a scope Effect remembers nothing (gorge reads no ExileOnLost$)
		ps.consume("ExileOnLost")
	}
	if eoc {
		// Suppressor Skyguard: the record ends with the combat, not the turn
		bits |= kop.PrevScopeEndCombat
		ps.consume("Duration", "UntilEndOfCombat")
	}
	ps.consume("ReplacementEffects", "AILogic")
	in := lInstr{Op: "PreventDamageScope", C: bits, F: sf, F2: tf, Note: "prevent the damage the controller and their creatures would take this turn"}
	if err := lc.scopeFilterAgrees(in); err != nil { // lower_prevscopefilter.go
		return true, err
	}
	lc.emit(in)
	return true, nil
}

// scopeTargetBits is a ValidTarget$ of the bit forms: none (any recipient),
// You, Creature.YouCtrl or both. ok is false for any other word.
func scopeTargetBits(vt string) (bits int, ok bool) {
	if vt == "" {
		return kop.PrevScopeAll, true
	}
	for _, w := range strings.Split(vt, ",") {
		switch strings.TrimSpace(w) {
		case "":
		case "You":
			bits |= kop.PrevScopePlayer
		case "Creature.YouCtrl":
			bits |= kop.PrevScopeCreatures
		default:
			return 0, false
		}
	}
	return bits, true
}

// scopeSourceBits is a ValidSource$ of the bit forms: none, Creature.
// inZoneBattlefield or a Card.<Colour> list. ok is false for any other word.
func scopeSourceBits(vs string) (bits int, ok bool) {
	switch vs {
	case "":
		return 0, true
	case "Creature.inZoneBattlefield":
		return kop.PrevScopeSrcCreature, true
	}
	var col kop.ColorMask
	for _, w := range strings.Split(vs, ",") {
		c, ok := scopeSourceColours[strings.TrimSpace(w)]
		if !ok {
			return 0, false
		}
		col |= c
	}
	return int(col) << kop.PrevScopeColourShift, true
}

// scopeSourceColours are the ValidSource$ words of a colour-filtered scoped
// prevention (Surge of Salvation: Card.Red,Card.Black).
var scopeSourceColours = map[string]kop.ColorMask{
	"Card.White": kop.ColW, "Card.Blue": kop.ColU, "Card.Black": kop.ColB,
	"Card.Red": kop.ColR, "Card.Green": kop.ColG,
}
