package kgen

import "strings"

// api ChangeTargets (CR 115.7; gorge effChangeTargets)
// and the mass Counter over Defined$ ValidStack.
//
// ChangeTargets lowers in three shapes only. A target slot over the stack
// (changeTargetsShapes: a spell or ability, a spell, an activated ability,
// each with or without exactly one distinct target, TgfSingleTgt), or
// Defined$ TriggeredSpellAbility inside a SpellCast trigger (the spell whose
// cast fired it), or Defined$ ValidStack SpellAbility with Optional$ True in
// a TurnFaceUp trigger (Boltbender's any-number walk, ExtChangeTargetsMass).
// Optional$ True is read on all three; DefinedMagnet$ Self with
// ChangeSingleTarget$ True (Spellskite, Hydroelectric Specimen: the source
// takes the first target it can, C bit 1) only beside a slot. Every other
// param (Chooser$, RandomTarget$, TargetRestriction$, Condition* ...) stays
// unread and refuses by name.
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
		if strings.TrimSpace(d) == "ValidStack SpellAbility" {
			// Boltbender: "choose new targets for any number of other spells
			// and/or abilities". The mass walk is the same per-entry redirect
			// over every stack entry but the resolving trigger, bottom first.
			if tr == nil || tr.Event != "TurnFaceUp" {
				return refuse("ChangeTargets Defined$ ValidStack SpellAbility outside a TurnFaceUp trigger")
			}
			if c != 1 {
				return refuse("ChangeTargets Defined$ ValidStack SpellAbility without Optional$ True")
			}
			if _, ok := ps.m["TargetType"]; ok {
				return refuse("ChangeTargets with both Defined$ and TargetType$")
			}
			lc.emit(lInstr{Op: "ChangeTargetsMass", C: c})
			return nil
		}
		if strings.TrimSpace(d) == "Targeted" {
			// Commandeer's DBChooseTargets: "You may choose new targets for
			// it" — the redirect subject is the chain's own spell target
			// declared upstream (the ControlSpell head), and gorge
			// effChangeTargets over Defined$ Targeted walks that stack
			// spell's root declaration and each announced SubAbility$ link
			// (the same per-entry walk as the trigger form's RefTriggerObj).
			t := lc.lastTarget()
			if len(lc.chainT) != 1 || t < 0 {
				return refuse("ChangeTargets Defined$ Targeted over %d target slots: the subject is the chain's one spell target (Commandeer)", len(lc.chainT))
			}
			tg := lc.ab.Targets[t]
			if !tg.Spell || tg.Zone != zStack || tg.StackAb {
				return refuse("ChangeTargets Defined$ Targeted of a slot that is not one spell on the stack")
			}
			if _, ok := ps.m["TargetType"]; ok {
				return refuse("ChangeTargets with both Defined$ and TargetType$")
			}
			lc.emit(lInstr{Op: "ChangeTargets", Obj: lRef{Kind: rTarget, N: t}, C: c})
			return nil
		}
		if strings.TrimSpace(d) != "TriggeredSpellAbility" || tr == nil || tr.Event != "SpellCast" {
			return refuse("ChangeTargets Defined$ %q outside a SpellCast trigger", d)
		}
		if _, ok := ps.m["TargetType"]; ok {
			return refuse("ChangeTargets with both Defined$ and TargetType$")
		}
		lc.emit(lInstr{Op: "ChangeTargets", Obj: lRef{Kind: rTriggered}, C: c})
		return nil
	}
	if _, ok := ps.m["ConditionTargetValidTargeting"]; ok {
		// Meddle: gorge does not evaluate the gate (effects.UnmodelledCondition
		// skips the redirect with a Note), so no table plays it
		return refuse("ChangeTargets ConditionTargetValidTargeting$: gorge skips the gated redirect, the card stays refused until gorge evaluates it")
	}
	tt := strings.TrimSpace(ps.str("TargetType"))
	shape, ok := changeTargetsShapes[tt]
	if !ok {
		return refuse("ChangeTargets TargetType %q", tt)
	}
	mag, hasMag := ps.get("DefinedMagnet")
	sgl, hasSgl := ps.get("ChangeSingleTarget")
	if hasMag != hasSgl || hasMag && (strings.TrimSpace(mag) != "Self" || strings.TrimSpace(sgl) != "True") {
		return refuse("ChangeTargets DefinedMagnet$ %q beside ChangeSingleTarget$ %q (only Self beside True)", mag, sgl)
	}
	if hasMag {
		c |= 2
	}
	vt := ps.str("ValidTgts")
	if shape.spellF != "" {
		if strings.TrimSpace(vt) != "Card" {
			return refuse("ChangeTargets ValidTgts %q", vt)
		}
		vt = shape.spellF
	} else if shape.spell {
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
	spell   bool   // a spell only (TgtSpell), judged by ValidTgts$
	actOnly bool   // an activated ability only, no spell, no triggered ability
	single  bool   // exactly one distinct target (the singleTarget property)
	spellF  string // with spell: the spell filter the TargetType$ tokens name (ValidTgts$ is then plain Card)
}

// changeTargetsShapes are the TargetType$ values the redirect slot lowers, by
// gorge's token kinds (state.StackKindTokenOf): SpellAbility and Spell,
// Activated,Triggered name the spell and both ability kinds.
var changeTargetsShapes = map[string]changeTargetsShape{
	"SpellAbility.singleTarget":                 {single: true},                                         // Bolt Bend, Redirect Lightning
	"Spell,Activated,Triggered":                 {},                                                     // Deflecting Swat
	"Spell.singleTarget":                        {spell: true, single: true},                            // Misdirection, Imp's Mischief
	"Spell":                                     {spell: true},                                          // Sideswipe
	"Instant.singleTarget,Sorcery.singleTarget": {spell: true, single: true, spellF: "Instant,Sorcery"}, // Hydroelectric Specimen
	"Activated.singleTarget":                    {actOnly: true, single: true},                          // Reroute
}
