package kgen

import "strings"

// api ChangeTargets (CR 115.7; gorge effChangeTargets)
// and the mass Counter over Defined$ ValidStack.
//
// ChangeTargets lowers in two shapes only. A target slot over the stack
// (changeTargetsShapes: a spell or ability, a spell, an activated ability,
// each with or without exactly one distinct target, TgfSingleTgt), or
// Defined$ TriggeredSpellAbility inside a SpellCast trigger (the spell whose
// cast fired it). Optional$ True is the only other
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
	tt := strings.TrimSpace(ps.str("TargetType"))
	shape, ok := changeTargetsShapes[tt]
	if !ok {
		return refuse("ChangeTargets TargetType %q", tt)
	}
	vt := ps.str("ValidTgts")
	if shape.spell {
		// the spell's own card: a plain spell filter (Sideswipe's Card.Arcane)
		vt = strings.TrimSpace(vt)
	} else if vt != "Card,Emblem" && vt != "Card" {
		// gorge's stack census: any ability, a spell matching the spec (Card,Emblem
		// names every source; the Counter slots read the same)
		return refuse("ChangeTargets ValidTgts %q", vt)
	} else {
		vt = "Card"
	}
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true, StackAb: !shape.spell, ActOnly: shape.actOnly, AbOnly: shape.actOnly, SingleTgt: shape.single})
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

// changeTargetsShape is what a ChangeTargets slot admits on the stack.
type changeTargetsShape struct {
	spell   bool // a spell only (TgtSpell), judged by ValidTgts$
	actOnly bool // an activated ability only, no spell, no triggered ability
	single  bool // exactly one distinct target (the singleTarget property)
}

// changeTargetsShapes are the TargetType$ values the redirect slot lowers, by
// gorge's token kinds (state.StackKindTokenOf): SpellAbility and Spell,
// Activated,Triggered name the spell and both ability kinds.
var changeTargetsShapes = map[string]changeTargetsShape{
	"SpellAbility.singleTarget": {single: true},                // Bolt Bend, Redirect Lightning
	"Spell,Activated,Triggered": {},                            // Deflecting Swat
	"Spell.singleTarget":        {spell: true, single: true},   // Misdirection, Imp's Mischief
	"Spell":                     {spell: true},                 // Sideswipe
	"Activated.singleTarget":    {actOnly: true, single: true}, // Reroute
}
