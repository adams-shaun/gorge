package kgen

// api PreventDamage (CR 615.7; gorge effects/prevent_damage.go
// effPreventDamage) as OpExt ExtPreventNext: "prevent the next N damage that
// would be dealt to <one recipient> this turn". The recipient is the ability's
// one target (any target, a creature, a player, a filtered creature) or its
// Defined$ You / Self; N is Amount$ read once as the effect resolves. The
// shield depletes across damage events and lets the rest through
// (dirac/repl_prevnext.go).
//
// Shapes the kernel has no record for stay refused, each with its true reason
// (the refusal class is still api:PreventDamage): a divided total over any
// number of targets, a set of recipients (Defined$ Valid ..., Radiance), a
// random recipient, a rider run per application, and CombatDamage$ True
// (gorge's effPreventDamage does not read it: the shield it installs prevents
// noncombat damage too, which the card does not say).
func (lc *lowerCtx) preventNext(ps *params) error {
	for _, k := range []struct{ key, why string }{
		{"DividedAsYouChoose", "a total divided among any number of targets"},
		{"TargetMax", "more than one recipient"},
		{"TargetMin", "a recipient count"},
		{"Radiance", "a shield for every creature sharing a colour with the target"},
		{"Random", "a random recipient"},
		{"PreventionSubAbility", "a rider run at each application"},
		{"CombatDamage", "a combat-damage-only shield (gorge installs no such filter)"},
	} {
		if _, ok := ps.m[k.key]; ok {
			return refuse("api PreventDamage: %s: %s", k.key, k.why)
		}
	}
	_, tgt := ps.m["ValidTgts"]
	if d, ok := ps.m["Defined"]; ok {
		if tgt {
			return refuse("api PreventDamage: Defined$ %q beside ValidTgts$", d)
		}
		if d = ps.peek("Defined"); d != "You" && d != "Self" {
			return refuse("api PreventDamage: Defined$ %q is a set of recipients", d)
		}
	}
	n, err := lc.amount(ps, "Amount", 0)
	if err != nil {
		return err
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget && obj.Kind != rYou && obj.Kind != rSelf {
		return refuse("api PreventDamage: recipient %s", obj)
	}
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, A: n, Note: "prevent the next A damage that would be dealt to Obj this turn"})
	return nil
}
