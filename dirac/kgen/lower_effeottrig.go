package kgen

import (
	"reflect"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// eotCombatEmblemRefuse refuses the until-end-of-turn Effect triggers on
// combat events the kernel's emblem hooks do not carry exactly
// (dirac/trig_effectuntileot.go; gorge effEffect registers the body as a
// recurring delayed trigger, trigger_delayed.go delayedEventMatches). A
// DamageDone emblem fires with the damage dealt (TriggerCount$DamageAmount),
// and its ValidTarget$ is exactly Player or You (a mixed player-and-object
// target falls to build's second-filter refusal; You is Tf2ToYou, read for
// the emblem's owner). An AttackersDeclared one counts
// every attacker of the declaration.
func eotCombatEmblemRefuse(ab *lAbility) error {
	if t := ab.Trigger; t != nil && (t.Event == "DamageDone" || t.Event == "DamageDealtOnce" || t.Event == "AttackersDeclared" || t.Event == "Attacks") {
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
	if ab.Trigger != nil && ab.Trigger.Event == "DamageDealtOnce" && ab.Trigger.F2 != nil {
		// the hook fires for any recipient and carries no recipient filter
		return refuse("Effect DamageDealtOnce trigger with ValidTarget")
	}
	if ab.Trigger == nil || ab.Trigger.Event != "DamageDone" {
		return nil
	}
	f2 := ab.Trigger.F2
	if f2 == nil || f2.Players == 0 || len(f2.Clauses) == 0 && f2.Players != plBoth && f2.Players != plYou {
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
	if strings.TrimSpace(ro) != "Targeted" || !eot || t.Mode != "DamageDone" && t.Mode != "DamageDealtOnce" {
		return lRef{}, false, refuse("Effect triggers with RememberObjects$ %q", ro)
	}
	var ref lRef
	if lc.pairCap != nil {
		ref = *lc.pairCap // resolved once by the static-and-trigger pair
	} else if _, ok := ps.m["ValidTgts"]; !ok {
		var err error
		if ref, err = lc.parentCapture(); err != nil {
			return lRef{}, false, err
		}
	} else {
		var err error
		if ref, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return lRef{}, false, err
		}
	}
	if ref.Kind != rTarget {
		return lRef{}, false, refuse("Effect RememberObjects$ Targeted over %s", ref)
	}
	ps.consume("RememberObjects")
	return ref, true, nil
}

// parentCapture is an Effect link's RememberObjects$ Targeted with no target
// of its own: gorge remembers the union of every target choice down the
// chain (effectRemembered, refTargetUnion), so the capture is the chain's
// one battlefield object target (Captain Howler: the pump's creature). A
// chain with several targets, or a player or non-battlefield one, refuses.
func (lc *lowerCtx) parentCapture() (lRef, error) {
	if len(lc.chainT) != 1 {
		return lRef{}, refuse("Effect RememberObjects$ Targeted without ValidTgts$ over %d chain targets", len(lc.chainT))
	}
	ti := lc.chainT[0]
	ts := lc.ab.Targets[ti]
	if ts.F == nil || ts.F.Players != 0 || len(ts.F.Clauses) == 0 || ts.Zone != zBattlefield || !ts.Max.isConst() || ts.Max.N != 1 {
		return lRef{}, refuse("Effect RememberObjects$ Targeted without ValidTgts$: the parent target is not one battlefield object")
	}
	return lRef{Kind: rTarget, N: ti}, nil
}

// emblemCaptureFilter strips the Remembered flag from a captured emblem's
// DamageDone source filter: the kernel matches the emblem's captured object
// as the damage source instead (Emblem.Capture), so the remaining filter is
// the type check. Any other Remembered read in the trigger refuses.
func emblemCaptureFilter(ab *lAbility) error {
	t := ab.Trigger
	if t == nil || t.Event != "DamageDone" && t.Event != "DamageDealtOnce" || t.F == nil || len(t.F.Clauses) != 1 || t.F.Players != 0 ||
		t.F.Clauses[0].Flags&fRemembered == 0 {
		return refuse("Effect RememberObjects$ Targeted beside a trigger that is not ValidSource$ ...IsRemembered")
	}
	f := *t.F
	f.Clauses = append([]lClause(nil), f.Clauses...)
	f.Clauses[0].Flags &^= fRemembered
	t.F = &f
	// a DamageDealtOnce body may read the set its own links wrote (lowerCtx.capRem
	// made the first read the damage source); a read before any write is the
	// capture the kernel does not seed
	written := false
	for _, in := range ab.Prog {
		if kremReads(in.String()) && !written {
			return refuse("Effect trigger body reads Remembered beside RememberObjects$ Targeted")
		}
		written = written || t.Event == "DamageDealtOnce" && kremWrites(in)
	}
	return nil
}

// effectSourceTrigger reads an until-end-of-turn Effect trigger whose
// ValidSource$ is exactly Card.EffectSource on a DamageDone event (Lyra,
// Tolarian Archangel: "whenever it deals combat damage to a player this
// turn"). gorge's EffectSource is the Effect's source object (a stable id: a
// source that left and came back is still it); the kernel's capture emblem
// remembers an object and fires only for it as the damage source, so the
// filter is rewritten to the remembered spelling and the caller captures the
// source (rSelf). found reports the rewrite. Any other EffectSource
// spelling or key in the trigger body refuses; nothing else is touched.
func effectSourceTrigger(ps *params, t cards.Trigger) (cards.Trigger, bool, error) {
	has := false
	keys := make([]string, 0, len(t.Params))
	for k := range t.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys) // the refusal text reaches the census: name the same key every run
	for _, k := range keys {
		v := t.Params[k]
		if strings.Contains(v, "EffectSource") {
			has = true
			if k != "ValidSource" || v != "Card.EffectSource" || t.Mode != "DamageDone" {
				return t, false, refuse("Effect trigger %s %s$ %q", t.Mode, k, v)
			}
		}
	}
	if !has {
		return t, false, nil
	}
	if _, ok := ps.m["RememberObjects"]; ok {
		return t, false, refuse("Effect trigger ValidSource$ Card.EffectSource beside RememberObjects$")
	}
	cp := make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		cp[k] = v
	}
	cp["ValidSource"] = "Card.IsRemembered"
	t.Params = cp
	return t, true, nil
}
