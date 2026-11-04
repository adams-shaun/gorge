package kgen

import (
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// eotCombatEmblemRefuse refuses the until-end-of-turn Effect triggers on
// combat events the kernel's emblem hooks do not carry exactly
// (dirac/trig_effectuntileot.go; gorge effEffect registers the body as a
// recurring delayed trigger, trigger_delayed.go delayedEventMatches). A
// DamageDone emblem fires with the damage dealt (TriggerCount$DamageAmount),
// and its ValidTarget$ is exactly Player (a mixed player-and-object target
// falls to build's second-filter refusal). An AttackersDeclared one counts
// every attacker of the declaration.
func eotCombatEmblemRefuse(ab *lAbility) error {
	if t := ab.Trigger; t != nil && (t.Event == "DamageDone" || t.Event == "AttackersDeclared" || t.Event == "Attacks") {
		// the hooks read only these fields: any other trigger rider (an
		// intervening if, once per turn, your turn only, a batch, ...)
		// would be dropped
		z := *t
		z.Event, z.F, z.F2, z.Whose, z.Zone, z.Combat, z.Optional, z.MinAttackers, z.SrcInPlay, z.DefPlayer = "", nil, nil, 0, 0, 0, false, 0, false, 0
		if t.Event == "Attacks" {
			z.Alone, z.Attacked = false, 0 // the per-attacker hook reads both (fireAttacksGen)
		}
		if !reflect.DeepEqual(z, lTrigger{}) {
			return refuse("Effect %s trigger rider", t.Event)
		}
	}
	if ab.Trigger != nil && ab.Trigger.Event == "AttackersDeclared" && ab.Trigger.DefPlayer == 0 && ab.Trigger.F != nil {
		// without a defender line the kernel's emblem hook counts every
		// attacker of the one declaration (fireEmblemsAttack): no
		// ValidAttackers$; the per-defender line is walked with the
		// declaration's groups (fireAttacksGen)
		return refuse("Effect AttackersDeclared trigger with ValidAttackers or AttackedTarget")
	}
	if ab.Trigger == nil || ab.Trigger.Event != "DamageDone" {
		return nil
	}
	f2 := ab.Trigger.F2
	if f2 == nil || f2.Players == 0 || len(f2.Clauses) == 0 && f2.Players != plBoth {
		return refuse("Effect DamageDone trigger ValidTarget other than Player")
	}
	return nil
}

// emblemCapture reads an Effect's RememberObjects$ Targeted for the one
// shape the kernel's emblem remembers: an until-end-of-turn DamageDone
// trigger over its own ValidTgts$ creature slot, the trigger's source filter
// being exactly that remembered object (Creature.IsRemembered). It declares
// the target slot on the chain and returns it; every other RememberObjects$
// beside an Effect trigger refuses (the kernel's remembered set is
// per resolution, so the trigger would read an empty set).
func (lc *lowerCtx) emblemCapture(ps *params, t cards.Trigger, eot bool) (lRef, bool, error) {
	ro, ok := ps.m["RememberObjects"]
	if !ok {
		return lRef{}, false, nil
	}
	if strings.TrimSpace(ro) != "Targeted" || !eot || t.Mode != "DamageDone" {
		return lRef{}, false, refuse("Effect triggers with RememberObjects$ %q", ro)
	}
	if _, ok := ps.m["ValidTgts"]; !ok {
		return lRef{}, false, refuse("Effect RememberObjects$ Targeted without ValidTgts$")
	}
	ref, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return lRef{}, false, err
	}
	if ref.Kind != rTarget {
		return lRef{}, false, refuse("Effect RememberObjects$ Targeted over %s", ref)
	}
	ps.consume("RememberObjects")
	return ref, true, nil
}

// emblemCaptureFilter strips the Remembered flag from a captured emblem's
// DamageDone source filter: the kernel matches the emblem's captured object
// as the damage source instead (Emblem.Capture), so the remaining filter is
// the type check. Any other Remembered read in the trigger refuses.
func emblemCaptureFilter(ab *lAbility) error {
	t := ab.Trigger
	if t == nil || t.Event != "DamageDone" || t.F == nil || len(t.F.Clauses) != 1 || t.F.Players != 0 ||
		t.F.Clauses[0].Flags&fRemembered == 0 {
		return refuse("Effect RememberObjects$ Targeted beside a trigger that is not ValidSource$ ...IsRemembered")
	}
	f := *t.F
	f.Clauses = append([]lClause(nil), f.Clauses...)
	f.Clauses[0].Flags &^= fRemembered
	t.F = &f
	for _, in := range ab.Prog {
		if kremReads(in.String()) {
			return refuse("Effect trigger body reads Remembered beside RememberObjects$ Targeted")
		}
	}
	return nil
}
