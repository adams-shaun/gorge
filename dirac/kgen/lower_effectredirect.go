package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// effectDmgRedirectNext lowers the en-Kor Effect (Nomads en-Kor, Warrior
// en-Kor, the corpus's Lancers/Outrider/Shaman/Spirit en-Kor): AB$ Effect |
// ValidTgts$ Creature.YouCtrl | RememberObjects$ Targeted | ExileOnMoved$
// Battlefield | Duration$ UntilHostLeavesPlayOrEOT over one Event$ DamageDone |
// ValidTarget$ Creature.EffectSource | DamageTarget$ Remembered replacement
// whose body is DB$ ReplaceSplitDamage | DamageTarget$ Remembered (gorge
// replacement_splitdamage.go: the next 1 damage to the host is dealt to the
// remembered creature instead). One OpExt ExtDmgRedirect over the target
// (dirac/repl_dmgredirect.go). Every other shape refuses by name.
func (lc *lowerCtx) effectDmgRedirectNext(ps *params) error {
	rp, err := lc.redirectLine(ps)
	if err != nil {
		return err
	}
	if !rp.is("ValidTarget", "Creature.EffectSource") {
		return refuse("Effect DamageDone redirect ValidTarget$ %q", rp.str("ValidTarget"))
	}
	if err := lc.redirectBody(rp, "ReplaceSplitDamage", 2); err != nil {
		return err
	}
	if ps.str("RememberObjects") != "Targeted" {
		return refuse("Effect DamageDone redirect RememberObjects %q", ps.str("RememberObjects"))
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return refuse("Effect DamageDone redirect without a target")
	}
	if f, err := lc.filter(ps.str("ValidTgts")); err != nil {
		return err
	} else if f.Players != 0 {
		// the registration redirects to a permanent (Zhalfirin Crusader's Any also names players)
		return refuse("Effect DamageDone redirect ValidTgts$ %q names a player", ps.str("ValidTgts"))
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	ps.consume("ReplacementEffects", "RememberObjects", "Duration", "ExileOnMoved", "Name")
	lc.emit(lInstr{Op: "DmgRedirect", Obj: obj, C: kop.DmgRedirectNext, Note: "the next 1 damage to the source this turn is dealt to the target instead"})
	return nil
}

// effectDmgRedirectScope lowers Gideon's Sacrifice's Effect: DB$ Effect |
// RememberObjects$ ChosenCard | ExileOnMoved$ Battlefield over one Event$
// DamageDone | ActiveZones$ Command | ValidTarget$ You,Permanent.YouCtrl |
// DamageTarget$ Remembered replacement whose body is DB$ ReplaceEffect |
// VarName$ Affected | VarValue$ Remembered | VarType$ Card (gorge
// replacement_replace.go's Affected rewrite). It reports false for a shape that
// is no redirect so effectPrevent's own arms read it.
func (lc *lowerCtx) effectDmgRedirectScope(ps *params) (bool, error) {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "DamageDone" || r["DamageTarget"] == "" {
		return false, nil
	}
	rp, err := lc.redirectLine(ps)
	if err != nil {
		return true, err
	}
	if !rp.is("ActiveZones", "Command") || !rp.is("ValidTarget", "You,Permanent.YouCtrl") {
		return true, refuse("Effect DamageDone redirect ValidTarget$ %q", rp.str("ValidTarget"))
	}
	if err := lc.redirectBody(rp, "ReplaceEffect", 4); err != nil {
		return true, err
	}
	body := etbParams(lc.face.SVars[r["ReplaceWith"]])
	if body["VarName"] != "Affected" || body["VarValue"] != "Remembered" || body["VarType"] != "Card" {
		return true, refuse("Effect DamageDone redirect body")
	}
	if ps.str("RememberObjects") != "ChosenCard" {
		return true, refuse("Effect DamageDone redirect RememberObjects %q", ps.str("RememberObjects"))
	}
	if v, ok := ps.get("ExileOnMoved"); !ok || strings.TrimSpace(v) != "Battlefield" {
		return true, refuse("Effect DamageDone redirect without ExileOnMoved$ Battlefield")
	}
	obj, err := lc.defined("ChosenCard")
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "RememberObjects", "ExileOnMoved")
	lc.emit(lInstr{Op: "DmgRedirect", Obj: obj, C: kop.DmgRedirectScope, Note: "all damage to the controller and their permanents this turn is dealt to the chosen permanent instead"})
	return true, nil
}

// effectDmgRedirectToHost lowers Daughter of Autumn's Effect: AB$ Effect |
// Cost$ W | ValidTgts$ Creature.White | RememberObjects$ Targeted |
// ExileOnMoved$ Battlefield | Duration$ UntilHostLeavesPlayOrEOT over one
// Event$ DamageDone | ValidTarget$ Creature.IsRemembered | DamageTarget$
// EffectSource | ReplaceWith$ line whose body is DB$ ReplaceSplitDamage |
// DamageTarget$ EffectSource (gorge replacement_splitdamage.go: the next 1
// damage to the remembered creature is dealt to the host instead). One OpExt
// ExtDmgRedirect of the kop.DmgRedirectHost kind, Obj2 naming the host
// (dirac/repl_dmgredirect.go): DmgRedirectNext with host and recipient
// swapped. It reports false, consuming nothing, for any shape it does not
// read, so the en-Kor arm's refusals stand.
func (lc *lowerCtx) effectDmgRedirectToHost(ps *params) (bool, error) {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "DamageDone" ||
		strings.TrimSpace(r["ValidTarget"]) != "Creature.IsRemembered" ||
		strings.TrimSpace(r["DamageTarget"]) != "EffectSource" {
		return false, nil
	}
	rp := newParams(r)
	rp.consume("Event", "Description", "ValidTarget", "DamageTarget")
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if len(body) != 2 || body["DB"] != "ReplaceSplitDamage" || body["DamageTarget"] != "EffectSource" {
		return false, nil
	}
	if err := lc.redirectBody(rp, "ReplaceSplitDamage", 2); err != nil {
		return true, err
	}
	if ps.str("RememberObjects") != "Targeted" {
		return true, refuse("Effect DamageDone redirect RememberObjects %q", ps.str("RememberObjects"))
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return true, refuse("Effect DamageDone redirect without a target")
	}
	if f, err := lc.filter(ps.str("ValidTgts")); err != nil {
		return true, err
	} else if f.Players != 0 {
		// a player host is no battlefield record (the damage would have to be
		// dealt to a player: the redirect records name permanents)
		return true, refuse("Effect DamageDone redirect ValidTgts$ %q names a player", ps.str("ValidTgts"))
	}
	if v, ok := ps.get("ExileOnMoved"); !ok || strings.TrimSpace(v) != "Battlefield" {
		return true, refuse("Effect DamageDone redirect without ExileOnMoved$ Battlefield")
	}
	host, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "RememberObjects", "Duration", "ExileOnMoved", "Name")
	lc.emit(lInstr{Op: "DmgRedirect", Obj2: host, C: kop.DmgRedirectHost, Note: "the next 1 damage to the target this turn is dealt to the host instead"})
	return true, nil
}

// effectDmgRedirectToYou lowers the no-Duration redirect-to-controller
// Effects: Vassal's Duty (AB$ Effect | ValidTgts$ Creature.Legendary+YouCtrl |
// RememberObjects$ Targeted | ExileOnMoved$ Battlefield over one Event$
// DamageDone | ValidTarget$ Creature.IsRemembered | DamageTarget$ You whose
// body is DB$ ReplaceSplitDamage | DamageTarget$ You: the next 1 damage to the
// remembered creature this turn is dealt to the resolving controller) and
// Sivvi's Valor (SP$ Effect | ValidTgts$ Creature | RememberObjects$ Targeted
// | ForgetOnMoved$ Battlefield over one Event$ DamageDone | ValidTarget$
// Card.IsRemembered | DamageTarget$ You whose body is DB$ ReplaceEffect |
// VarName$ Affected | VarValue$ You | VarType$ Player: all of it, this turn)
// (gorge replacement_splitdamage.go, replacement_replace.go's Affected
// rewrite). One OpExt ExtDmgRedirect over the remembered target, C the kind
// (dirac/repl_dmgredirect.go). It reports false, consuming nothing, for any
// other shape, so the other arms' refusals stand.
func (lc *lowerCtx) effectDmgRedirectToYou(ps *params) (bool, error) {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "DamageDone" || strings.TrimSpace(r["DamageTarget"]) != "You" {
		return false, nil
	}
	switch strings.TrimSpace(r["ValidTarget"]) {
	case "Creature.IsRemembered", "Card.IsRemembered":
	default:
		return false, nil
	}
	body := etbParams(lc.face.SVars[r["ReplaceWith"]])
	var kind int
	switch {
	case len(body) == 2 && body["DB"] == "ReplaceSplitDamage" && body["DamageTarget"] == "You":
		kind = kop.DmgRedirectYouNext // the next 1 damage (Vassal's Duty)
	case len(body) == 4 && body["DB"] == "ReplaceEffect" && body["VarName"] == "Affected" &&
		body["VarValue"] == "You" && body["VarType"] == "Player":
		kind = kop.DmgRedirectYouAll // all of it, this turn (Sivvi's Valor)
	default:
		return false, nil
	}
	if ps.str("RememberObjects") != "Targeted" {
		return false, nil
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return false, nil
	}
	if _, ok := ps.get("Duration"); ok {
		return false, nil // effectDurationRoutes owns every Duration$ lifetime
	}
	// the record ends when the remembered host leaves: the Effect's
	// ExileOnMoved$/ForgetOnMoved$ Battlefield, exactly one of the two
	ex, hasEx := ps.get("ExileOnMoved")
	fg, hasFg := ps.get("ForgetOnMoved")
	switch {
	case hasEx && !hasFg && strings.TrimSpace(ex) == "Battlefield":
	case hasFg && !hasEx && strings.TrimSpace(fg) == "Battlefield":
	default:
		return false, nil
	}
	rp := newParams(r)
	rp.consume("Event", "Description", "ValidTarget", "ActiveZones", "DamageTarget", "ReplaceWith")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	if f, err := lc.filter(ps.str("ValidTgts")); err != nil {
		return true, err
	} else if f.Players != 0 {
		// the damaged host is a battlefield record (a player ValidTgts would
		// have the damage dealt to a player's own event: not modelled)
		return false, nil
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "RememberObjects", "Duration", "ExileOnMoved", "ForgetOnMoved", "Name")
	note := "all damage to the target this turn is dealt to the controller instead"
	if kind == kop.DmgRedirectYouNext {
		note = "the next 1 damage to the target this turn is dealt to the controller instead"
	}
	lc.emit(lInstr{Op: "DmgRedirect", Obj: obj, C: kind, Note: note})
	return true, nil
}

// redirectLine reads the Effect's one DamageDone replacement line and checks
// the parts both redirects share: DamageTarget$ Remembered, and that nothing
// else is on the line. The returned params still hold ValidTarget$,
// ActiveZones$ and ReplaceWith$ for the caller.
func (lc *lowerCtx) redirectLine(ps *params) (*params, error) {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return nil, refuse("Effect DamageDone redirect replacement list %q", name)
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "DamageDone" {
		return nil, refuse("Effect replacement %q", name)
	}
	rp := newParams(r)
	rp.consume("Event", "Description")
	if !rp.is("DamageTarget", "Remembered") {
		return nil, refuse("Effect DamageDone redirect DamageTarget$ %q", rp.str("DamageTarget"))
	}
	return rp, nil
}

// redirectBody checks the line's ReplaceWith$ body is the one DB$ api with n
// parameters (api, DamageTarget$ or VarName$/VarValue$/VarType$).
func (lc *lowerCtx) redirectBody(rp *params, api string, n int) error {
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if body == nil || len(body) != n || body["DB"] != api {
		return refuse("Effect DamageDone redirect body")
	}
	rp.consume("ReplaceWith", "ValidTarget", "ActiveZones", "DamageTarget")
	if left := rp.unread(); len(left) > 0 {
		return refuse("Effect DamageDone redirect unread %v", left)
	}
	return nil
}
