package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// resolvedLimit reads a trigger's ResolvedLimit$ (gorge trigger_match.go
// resolvedLimitValue / noteTriggerResolved): only the value 1 of a
// battlefield trigger is lowered (kop.Tf2ResolvedLimit: the kernel keys the
// count by the source object in its once-per-turn records, gated where a
// listener queues a trigger); any other value, and a trigger of an Effect
// or another zone, refuses.
func (lc *lowerCtx) resolvedLimit(tr *lTrigger, rl, zone string) error {
	if rl != "1" {
		return refuse("ResolvedLimit %q", rl)
	}
	if zone != "" || (tr.Zone != 0 && tr.Zone != zBattlefield) {
		return refuse("ResolvedLimit outside a battlefield trigger")
	}
	tr.ResolvedLimit = true
	return nil
}

// readEvolveCondition reads this family's trigger parameters. Condition$ Evolve
// (CR 702.100a written as a bare trigger condition; gorge trigger_match.go
// evolveConditionHolds) and Evolve$ True (K:Evolve's expansion, gorge
// kwEvolve; trigmatch zone.go): an ETB trigger fires only when the entering
// creature's power or toughness is greater than the source's, read as it
// fires and again as it resolves (CR 603.4, gorge evolveResolves). Any other
// Condition$ or Evolve$ value is left unread (refused by the caller).
func (lc *lowerCtx) readEvolveCondition(tp *params, tr *lTrigger) error {
	if c, ok := tp.get("Condition"); ok && strings.TrimSpace(c) == "Evolve" {
		if tr.Event != "ETB" {
			return refuse("Condition$ Evolve on a %s trigger", tr.Event)
		}
		tp.consume("Condition")
		tr.Evolve = true
	}
	if v, ok := tp.get("Evolve"); ok && strings.TrimSpace(v) == "True" {
		if tr.Event != "ETB" {
			return refuse("Evolve$ on a %s trigger", tr.Event)
		}
		tp.consume("Evolve")
		tr.Evolve = true
	}
	return nil
}

// trigParams3 reads three trigger parameters.
//
//   - Teamwork$ True (Taps, Card.Self): gorge trigmatch/tap.go:41 matches
//     only a Tap whose reason is events.TapTeamworkCounter, the creatures
//     elected for the teamwork additional cost (pay/paidcost.go:229-237).
//   - GameActivationLimit$ 1: gorge trigger_match.go:610-660, once per
//     source object (a zone change starts a new stint, CR 400.7).
//   - TriggerController$ TriggeredCardController: read by gorge only for a
//     ChangesZone/ChangesZoneAll line leaving the battlefield
//     (trigger_match.go:1789-1795); on any other mode it is inert.
func (lc *lowerCtx) trigParams3(tp *params, tr *lTrigger, mode string) error {
	if _, ok := tp.get("Teamwork"); ok {
		if mode != "Taps" || tp.str("Teamwork") != "True" || tr.Whose != 0 {
			return refuse("Teamwork$ on a %s trigger", mode)
		}
		tr.TeamworkTap = true
		tp.consume("Teamwork")
	}
	if v, ok := tp.get("GameActivationLimit"); ok {
		if strings.TrimSpace(v) != "1" {
			return refuse("GameActivationLimit$ %q", v)
		}
		if lc.granted != 0 {
			return refuse("GameActivationLimit$ on a granted trigger")
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("GameActivationLimit$ off the battlefield")
		}
		tr.GameOnce = true
		tp.consume("GameActivationLimit")
	}
	// peek, not get: any other value (or mode) must stay unread and refuse
	if v, ok := tp.m["TriggerController"]; ok && strings.TrimSpace(v) == "TriggeredCardController" &&
		mode != "ChangesZone" && mode != "ChangesZoneAll" {
		tp.consume("TriggerController")
	}
	return nil
}

// trigParams4 reads the trigger parameters:
//
//   - IsSingleTarget$ True (SpellCast): gorge trigmatch.TargetShapeMatches
//     (cast.go L200, L234-245) requires the cast spell's ROOT target list to
//     have exactly one entry; any other value fails closed in gorge, so kgen
//     refuses it rather than lowering a condition it does not read. A chained
//     SubAbility$ link's target rides obj.SubTargets and is not counted there
//     (the kernel's singleTargetOK reads the same root list).
//   - ValidCause$ Spell.numTargets EQ1 + TargetRelativeToCause$
//     Creature.targetedBy (DamageDone): gorge damageCauseMatches
//     (cause_qualifiers.go L71-101) reads the resolving stack object, which
//     must be a spell (stackCauseAdmits L24-40), carry exactly one target in
//     total (Targets plus SubTargets, targetCountAdmits L43-50), and the
//     damaged creature must be one of them. Only this exact pair is lowered;
//     any other spelling or a lone half refuses.
//   - Origin$ / Destination$ on Sacrificed / SacrificedOnce: gorge
//     sacrificedMatches (zone.go L207-238) reads ValidCard$, ValidPlayer$ and
//     WhileKeyword$ and never these two, so they are inert and consumed here.
func (lc *lowerCtx) trigParams4(tp *params, tr *lTrigger, mode string) error {
	if mode == "SpellCast" {
		if v, ok := tp.get("IsSingleTarget"); ok {
			if strings.TrimSpace(v) != "True" {
				return refuse("IsSingleTarget$ %q", v)
			}
			tr.SingleTarget = true
		}
	} else if v, ok := tp.m["IsSingleTarget"]; ok && !tp.used["IsSingleTarget"] {
		return refuse("IsSingleTarget$ %q on a %s trigger", v, mode)
	}
	if mode == "DamageDone" {
		vc, vok := tp.get("ValidCause")
		rc, rok := tp.get("TargetRelativeToCause")
		if vok || rok {
			if !vok || !rok || strings.TrimSpace(vc) != "Spell.numTargets EQ1" || strings.TrimSpace(rc) != "Creature.targetedBy" ||
				strings.TrimSpace(tp.str("ValidTarget")) != "Creature" {
				return refuse("DamageDone cause qualifiers ValidCause$ %q TargetRelativeToCause$ %q ValidTarget$ %q", vc, rc, tp.str("ValidTarget"))
			}
			tr.CauseTarget = true
		}
	} else {
		if v, ok := tp.m["ValidCause"]; ok && !tp.used["ValidCause"] && (mode == "LifeLost" || mode == "LifeLostAll") {
			// gorge trigmatch/life.go lifeLossCauseMatches: the ValidCause$
			// grammar of a LifeLost trigger is a comma list of
			// SpellAbility alternatives, each optionally qualified .YouCtrl
			// or .OppCtrl -- the loss was caused by a spell or ability the
			// listener's controller does or doesn't control, resolving at
			// the moment of the loss (gorge's Facts.Damaging). Combat
			// damage is never a spell-or-ability cause, and a loss with no
			// resolution in flight (a cost payment, a mana ability's cost)
			// matches none, as the kernel's gate reads the resolving entry
			// (kop.Tf4CauseSpellAb).
			if n, err := lc.lifeLossCause(v); err != nil {
				return err
			} else {
				tr.CauseSpell = n
			}
			tp.used["ValidCause"] = true
		}
		for _, k := range [...]string{"ValidCause", "TargetRelativeToCause"} {
			if v, ok := tp.m[k]; ok && !tp.used[k] {
				return refuse("%s$ %q on a %s trigger", k, v, mode)
			}
		}
	}
	if mode == "Sacrificed" || mode == "SacrificedOnce" {
		// gorge sacrificedMatches never reads Origin$/Destination$; they are
		// the zone-change frame's descriptive keys and are inert here too.
		tp.consume("Origin", "Destination")
	}
	return nil
}

// lifeLossCause reads a LifeLost trigger's ValidCause$ (gorge
// trigmatch/life.go lifeLossCauseMatches): a comma list whose every
// alternative is SpellAbility, optionally qualified .YouCtrl or .OppCtrl.
// Any unqualified alternative, or both qualifiers together, makes the whole
// gate any spell or ability (1); one qualifier only narrows it (2 you, 3
// the others).
func (lc *lowerCtx) lifeLossCause(v string) (uint8, error) {
	const (
		causeAny uint8 = 1 + iota
		causeYou
		causeOpp
	)
	any, you, opp := false, false, false
	for alt := range strings.SplitSeq(v, ",") {
		base, qual, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if base != "SpellAbility" {
			return 0, refuse("ValidCause$ %q on a LifeLost trigger", v)
		}
		switch qual {
		case "":
			any = true
		case "YouCtrl":
			you = true
		case "OppCtrl":
			opp = true
		default:
			return 0, refuse("ValidCause$ %q on a LifeLost trigger", v)
		}
	}
	switch {
	case any || you && opp:
		return causeAny, nil
	case you:
		return causeYou, nil
	case opp:
		return causeOpp, nil
	default:
		return 0, refuse("ValidCause$ %q on a LifeLost trigger", v)
	}
}

// lowerDrawnFirstInDrawStep reads a Drawn trigger's FirstCardInDrawStep$,
// which gates the draw's position in the drawer's own draw step (gorge
// trigmatch.firstCardInDrawStep, CR 504.1). Both spellings lower; any other
// value is refused by name rather than guessed.
func lowerDrawnFirstInDrawStep(tp *params, tr *lTrigger) error {
	v, ok := tp.get("FirstCardInDrawStep")
	if !ok {
		return nil
	}
	switch strings.TrimSpace(v) {
	case "True":
		tr.FirstInDrawStep = true
	case "False":
		tr.NotFirstInDrawStep = true
	default:
		return refuse("Drawn FirstCardInDrawStep %q", v)
	}
	return nil
}

// drawStepFlags maps a lowered trigger's draw-step gate to its kernel flags.
func drawStepFlags(t *lTrigger) kop.TrigFlags5 {
	var f kop.TrigFlags5
	if t.FirstInDrawStep {
		f |= kop.Tf5FirstCardInDrawStep
	}
	if t.NotFirstInDrawStep {
		f |= kop.Tf5NotFirstCardInDrawStep
	}
	if t.OptSrcCtrl {
		f |= kop.Tf5OptSrcCtrl
	}
	if t.TapsForMana {
		f |= kop.Tf5TapsForMana
	}
	if t.TapperNonActive {
		f |= kop.Tf5TapperNonActive
	}
	if t.AtkYouLastTurn {
		f |= kop.Tf5DmgAttackedYouLastTurn // lower_atklastturn.go
	}
	return f
}

// lowerDiscardedCause reads a Discarded trigger's ValidCause$ (gorge
// trigmatch.DiscardCauseAdmits): only a card's own "when a spell or ability
// an opponent controls causes you to discard this card" lowers, which the
// kernel decides at the discard from the resolving object's controller
// (kop.Tf5OppCause; a cost or cleanup discard has no cause). Any other
// cause spec, or one beside a watcher, stays refused.
func lowerDiscardedCause(tp *params, tr *lTrigger) error {
	v, ok := tp.get("ValidCause")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "SpellAbility.OppCtrl" || tp.str("ValidCard") != "Card.Self" {
		return refuse("Discarded ValidCause$ %q beside ValidCard$ %q", v, tp.str("ValidCard"))
	}
	tr.OppCause = true
	return nil
}
