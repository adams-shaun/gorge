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
