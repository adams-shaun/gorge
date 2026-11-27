package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterTriggered lowers api:Counter with Defined$ TriggeredSpellAbility
// in a SpellCast trigger ("counter that spell": gorge effCounter over the
// trigger's spell, which must still be on the stack and may be uncounterable)
// to OpCounter of the trigger object, the spell whose cast triggered it.
// On a BecomesTarget trigger, Defined$ TriggeredSourceSA counters the spell
// or ability that targeted the watched permanent (gorge TriggerStack, the
// kernel queue's trigSA slot; Kira, Great Glass-Spinner). Every other
// Defined$ refuses.
// counterTargeted lowers DB$ Counter | Defined$ Targeted cast as a spell:
// gorge effCounter over Defined (→ Targeted, the resolving chain's own
// targets, context.go definedSpecTargeted) counters the spell in the cast's
// one stack-spell slot — the slot the TargetOnly head declared (Disrupting
// Shoal's DBCounter sub; the kernel's execCounter resolves the slot's stack
// entry, act_counter.go). The SVar-condition gate on the node (Disrupting
// Shoal's ConditionCheckSVar$ T | ConditionSVarCompare$ EQX, T the targeted
// spell's mana value) is the node's own gate, lowered with it. A chain that
// is not exactly one spell slot is not this shape: false sends it to
// counterTriggered's refusal, which names the trigger it lacks.
func (lc *lowerCtx) counterTargeted(ps *params) (bool, error) {
	t := lc.lastTarget()
	if len(lc.chainT) != 1 || t < 0 {
		return false, nil
	}
	tg := lc.ab.Targets[t]
	if !tg.Spell || tg.Zone != zStack || tg.StackAb || !isConstN(tg.Min, 1) || !isConstN(tg.Max, 1) {
		return false, nil
	}
	dest, err := counterDestination(ps)
	if err != nil {
		return true, err
	}
	ps.consume("Defined")
	lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, C: dest})
	return true, nil
}

func (lc *lowerCtx) counterTriggered(d string) error {
	tr := lc.ab.Trigger
	if d == "TriggeredSourceSA" && tr != nil && tr.Event == "BecomesTarget" {
		lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTriggeredSource}})
		return nil
	}
	if d != "TriggeredSpellAbility" || tr == nil || tr.Event != "SpellCast" {
		return refuse("Counter Defined$ %q outside a SpellCast trigger", d)
	}
	lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTriggered}})
	return nil
}

// counterDestination reads Counter's Destination$ (gorge effCounter: the
// zone a countered spell CARD goes to instead of its graveyard; a flashback
// spell is still exiled). Battlefield and any other word refuse.
func counterDestination(ps *params) (int, error) {
	v, ok := ps.get("Destination")
	if !ok {
		return 0, nil
	}
	switch v {
	case "Graveyard":
		return 0, nil
	case "Exile":
		return kop.CtrDestExile, nil
	case "Hand":
		return kop.CtrDestHand, nil
	case "TopOfLibrary":
		return kop.CtrDestLibTop, nil
	case "BottomOfLibrary":
		return kop.CtrDestLibBottom, nil
	}
	return 0, refuse("Counter Destination %q", v)
}

// the Counter shapes beyond a plain "target spell":
// TargetType$ lists with a spell qualifier and abilities (Louisoix's
// Sacrifice, Spider-Sense), TargetValidTargeting$ (Fugitive Droid),
// RememberCounteredCMC$ (Hurl into History) and TgtZone$ Stack. Swallowed by
// Leviathan stays refused (gorge findings; see lowerCtx.unless).

// counterTypeList reads a TargetType$ list that is none of Counter's three
// exact forms (gorge state.StackKindTokenOf, OR over the tokens): one spell
// term set (Spell.nonCreature, or Instant and/or Sorcery) plus Triggered, with
// or without Activated. It returns the spell filter's expression and whether
// only triggered abilities (not activated ones) join the spells. Every other
// list refuses by name.
func counterTypeList(tt string) (spell string, trigOnly bool, err error) {
	var act, trig, inst, sorc, nonCre bool
	for _, tok := range strings.Split(tt, ",") {
		var seen *bool
		switch tok {
		case "Activated":
			seen = &act
		case "Triggered":
			seen = &trig
		case "Instant":
			seen = &inst
		case "Sorcery":
			seen = &sorc
		case "Spell.nonCreature":
			seen = &nonCre
		default:
			return "", false, refuse("Counter TargetType %q", tt)
		}
		if *seen {
			return "", false, refuse("Counter TargetType %q repeats a term", tt)
		}
		*seen = true
	}
	switch {
	case !trig:
		return "", false, refuse("Counter TargetType %q without Triggered", tt)
	case nonCre && (inst || sorc):
		return "", false, refuse("Counter TargetType %q mixes nonCreature with Instant/Sorcery", tt)
	case !nonCre && !inst && !sorc:
		return "", false, refuse("Counter TargetType %q without a spell term", tt)
	}
	switch {
	case nonCre:
		spell = "Card.nonCreature"
	case inst && sorc:
		spell = "Instant,Sorcery"
	case inst:
		spell = "Instant"
	default:
		spell = "Sorcery"
	}
	return spell, !act, nil
}

// counterValidTargeting reads TargetValidTargeting$ (gorge
// filterTargetValidTargeting, the offer and the CR 115.9b resolution
// recheck): every comma term must carry an
// inRealZoneBattlefield zone term, which is stripped (the kernel matches
// battlefield objects) and the rest built as a battlefield filter.
//
// The zone term is "+inRealZoneBattlefield" after a property (Permanent.YouCtrl)
// or ".inRealZoneBattlefield" after a bare type (Creature). The word "You"
// alone (gorge MatchesPlayerSpec: a target of the spell is the counter's
// controller; Dawn Charm) returns you with no filter. A term with no zone
// word (Mistfolk's Card.StrictlySelf, read in any zone) and any mix of "You"
// with a battlefield term (Siren Stormtamer) refuse.
func (lc *lowerCtx) counterValidTargeting(spec string) (f *lFilter, you bool, err error) {
	if spec == "You" {
		return nil, true, nil
	}
	const zone = "inRealZoneBattlefield"
	terms := strings.Split(spec, ",")
	for i, t := range terms {
		base, ok := strings.CutSuffix(t, "+"+zone)
		if !ok {
			base, ok = strings.CutSuffix(t, "."+zone)
		}
		if !ok || base == "" || strings.Contains(base, "inRealZone") {
			return nil, false, refuse("Counter TargetValidTargeting %q has no inRealZoneBattlefield term on every alternative (the kernel reads battlefield targets only; Mistfolk's any-zone Card.StrictlySelf, Siren Stormtamer's You beside a creature)", spec)
		}
		terms[i] = base
	}
	f, err = lc.filter(strings.Join(terms, ","))
	return f, false, err
}

// counterCMCWriter reports whether the program under construction has an
// earlier Counter with RememberCounteredCMC$ (Count$RememberedNumber then
// reads the countered spell's mana value, gorge count_body_paid.go).
func (lc *lowerCtx) counterCMCWriter() bool {
	for _, in := range lc.ab.Prog {
		if in.Op == "Counter" && in.B.isConst() && in.B.N&kop.CtrRemMV != 0 {
			return true
		}
	}
	return false
}
