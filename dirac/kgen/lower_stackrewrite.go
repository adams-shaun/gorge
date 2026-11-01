package kgen

import "strings"

// api ChangeTargets (CR 115.7; gorge effChangeTargets)
// and the mass Counter over Defined$ ValidStack.
//
// ChangeTargets lowers in two shapes only. A target slot, TargetType$
// SpellAbility.singleTarget with ValidTgts$ Card,Emblem (a spell or an
// ability with exactly one distinct target on the stack, TgtStackObj with
// TgfSingleTgt), or Defined$ TriggeredSpellAbility inside a SpellCast
// trigger (the spell whose cast fired it). Optional$ True is the only other
// param read; every other one (Chooser$, DefinedMagnet$, RandomTarget$,
// ChangeSingleTarget$, TargetRestriction$ ...) stays unread and refuses by
// name.
func (lc *lowerCtx) changeTargets(ps *params) error {
	c := 0
	if v, ok := ps.get("Optional"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("ChangeTargets Optional$ %q", v)
		}
		c = 1
	}
	if d, ok := ps.get("Defined"); ok {
		tr := lc.ab.Trigger
		if strings.TrimSpace(d) != "TriggeredSpellAbility" || tr == nil || tr.Event != "SpellCast" {
			return refuse("ChangeTargets Defined$ %q outside a SpellCast trigger", d)
		}
		if _, ok := ps.m["TargetType"]; ok {
			return refuse("ChangeTargets with both Defined$ and TargetType$")
		}
		lc.emit(lInstr{Op: "ChangeTargets", Obj: lRef{Kind: rTriggered}, C: c})
		return nil
	}
	if tt := ps.str("TargetType"); tt != "SpellAbility.singleTarget" {
		return refuse("ChangeTargets TargetType %q", tt)
	}
	if vt := ps.str("ValidTgts"); vt != "Card,Emblem" {
		return refuse("ChangeTargets ValidTgts %q", vt)
	}
	f, err := lc.filter("Card")
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true, StackAb: true, SingleTgt: true})
	lc.emit(lInstr{Op: "ChangeTargets", Obj: lRef{Kind: rTarget, N: t}, C: c})
	return nil
}

// counterAll lowers Counter Defined$ ValidStack <spec>: exactly the three
// opponent-controlled kinds, in gorge's token order (a spec with any other
// token or qualifier refuses; gorge's validStackTokens would read it, kgen
// does not). RememberCounteredSA$ True is required: the Count$RememberedSize
// after it reads the sweep's count.
func (lc *lowerCtx) counterAll(ps *params, spec string) error {
	if spec != "Spell.OppCtrl,Activated.OppCtrl,Triggered.OppCtrl" {
		return refuse("Counter Defined$ ValidStack %q", spec)
	}
	if !ps.is("RememberCounteredSA", "True") {
		return refuse("Counter Defined$ ValidStack without RememberCounteredSA$ True")
	}
	lc.emit(lInstr{Op: "CounterAll"})
	return nil
}
