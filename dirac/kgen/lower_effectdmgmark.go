package kgen

import "strings"

// effectDmgMark lowers the Effect that doubles the damage a player and their
// permanents take until the controller's next turn (Lightning, Army of One's
// Stagger): DB$ Effect | Duration$ UntilYourNextTurn | RememberObjects$
// TriggeredTarget over one Event$ DamageDone | ActiveZones$ Command |
// ValidSource$ Card,Emblem | ValidTarget$ Player.IsRemembered,Permanent.
// ControlledBy Player.IsRemembered replacement whose body is DB$ ReplaceEffect |
// VarName$ DamageAmount | VarValue$ ReplaceCount$DamageAmount/Twice (gorge
// replacement_damage.go reads Player.IsRemembered from the players the effect
// captured). One OpExt ExtDmgMark over the damaged player the damage trigger
// names (dirac/repl_dmgmark.go). Every other shape refuses by name.
func (lc *lowerCtx) effectDmgMark(ps *params) error {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return refuse("Effect DamageDone doubling replacement list %q", name)
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "DamageDone" {
		return refuse("Effect replacement %q", name)
	}
	rp := newParams(r)
	rp.consume("Event", "Description")
	if !rp.is("ActiveZones", "Command") {
		return refuse("Effect DamageDone doubling ActiveZones$ %q", rp.str("ActiveZones"))
	}
	if !rp.is("ValidSource", "Card,Emblem") {
		return refuse("Effect DamageDone doubling ValidSource$ %q", rp.str("ValidSource"))
	}
	if !rp.is("ValidTarget", "Player.IsRemembered,Permanent.ControlledBy Player.IsRemembered") {
		return refuse("Effect DamageDone doubling ValidTarget$ %q", rp.str("ValidTarget"))
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if body == nil || len(body) != 3 || body["DB"] != "ReplaceEffect" || body["VarName"] != "DamageAmount" {
		return refuse("Effect DamageDone doubling body")
	}
	vv := body["VarValue"]
	if sv, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(sv)
	}
	if vv != "ReplaceCount$DamageAmount/Twice" {
		return refuse("Effect DamageDone doubling VarValue$ %q", vv)
	}
	rp.consume("ReplaceWith")
	if left := rp.unread(); len(left) > 0 {
		return refuse("Effect DamageDone doubling unread %v", left)
	}
	if ro := ps.str("RememberObjects"); ro != "TriggeredTarget" {
		return refuse("Effect DamageDone doubling RememberObjects %q", ro)
	}
	if lc.ab.Kind != akTriggered {
		return refuse("Effect DamageDone doubling outside a triggered ability")
	}
	obj, err := lc.defined("TriggeredTarget")
	if err != nil {
		return err
	}
	if obj.Kind != rTriggeredPlayer {
		return refuse("Effect DamageDone doubling RememberObjects$ TriggeredTarget is no player")
	}
	ps.consume("ReplacementEffects", "RememberObjects", "Duration")
	lc.emit(lInstr{Op: "DmgMark", Obj: obj, Note: "until your next turn, damage to the damaged player and their permanents is doubled"})
	return nil
}
