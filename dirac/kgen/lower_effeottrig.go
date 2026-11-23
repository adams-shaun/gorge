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
func eotCombatEmblemRefuse(ab *lAbility, recipientCap bool) error {
	if t := ab.Trigger; t != nil && (t.Event == "DamageDone" || t.Event == "DamageDealtOnce" || t.Event == "AttackersDeclared" || t.Event == "Attacks" || t.Event == "AttackerBlocked" || t.Event == "Blocks") {
		// the hooks read only these fields: any other trigger rider (an
		// intervening if, once per turn, your turn only, ...) would be
		// dropped. Batch/PerRef are the line's own Mode$ DamageDoneOnce
		// lowering, not a rider the hook drops: it queues one trigger per
		// damage event with that event's amount.
		z := *t
		z.Event, z.F, z.F2, z.Whose, z.Zone, z.Combat, z.Optional, z.MinAttackers, z.SrcInPlay, z.DefPlayer = "", nil, nil, 0, 0, 0, false, 0, false, 0
		z.Batch, z.PerRef = false, false
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
	if f2 == nil {
		return refuse("Effect DamageDone trigger ValidTarget other than Player")
	}
	if f2.Players == 0 {
		// the recipient-captured shape (ValidTarget$ ...IsRemembered on the
		// damage RECIPIENT): the strip in emblemCaptureFilter leaves the
		// object filter, and the hook compares the damaged permanent with
		// the emblem's captured object
		if !recipientCap || len(f2.Clauses) == 0 {
			return refuse("Effect DamageDone trigger ValidTarget other than Player")
		}
		return nil
	}
	if len(f2.Clauses) == 0 && f2.Players != plBoth && f2.Players != plYou {
		return refuse("Effect DamageDone trigger ValidTarget other than Player")
	}
	return nil
}

// emblemCapture reads an Effect's RememberObjects$ Targeted/TargetedPlayer for
// the two captures the kernel's emblem can carry. Targeted is the one
// battlefield object target (the trigger's ValidSource$ ...IsRemembered is
// the capture); TargetedPlayer is the one player target (the trigger's
// ValidTarget$ Player.IsRemembered is rewritten to Player and the emblem's
// player capture scopes it to that seat). player reports the player form.
// Every other RememberObjects$ beside an Effect trigger refuses (the kernel's
// remembered set is per resolution, so the trigger would read an empty set).
func (lc *lowerCtx) emblemCapture(ps *params, t cards.Trigger, eot bool) (lRef, bool, bool, cards.Trigger, error) {
	ro, ok := ps.m["RememberObjects"]
	if !ok {
		return lc.chosenCardCapture(t, eot)
	}
	if strings.TrimSpace(ro) == "TargetedPlayer" {
		if !eot || t.Mode != "DamageDone" && t.Mode != "DamageDealtOnce" {
			return lRef{}, false, false, t, refuse("Effect triggers with RememberObjects$ %q", ro)
		}
		if ps.str("ValidTgts") != "Opponent" || strings.TrimSpace(t.Params["ValidTarget"]) != "Player.IsRemembered" {
			return lRef{}, false, false, t, refuse("Effect RememberObjects$ TargetedPlayer with ValidTgts$ %q and ValidTarget$ %q",
				ps.str("ValidTgts"), t.Params["ValidTarget"])
		}
		ref, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return lRef{}, false, false, t, err
		}
		if ref.Kind != rTarget {
			return lRef{}, false, false, t, refuse("Effect RememberObjects$ TargetedPlayer over %s", ref)
		}
		cp := make(map[string]string, len(t.Params))
		for k, v := range t.Params {
			cp[k] = v
		}
		cp["ValidTarget"] = "Player"
		t.Params = cp
		ps.consume("RememberObjects")
		return ref, true, true, t, nil
	}
	if strings.TrimSpace(ro) != "Targeted" || !eot || t.Mode != "DamageDone" && t.Mode != "DamageDoneOnce" && t.Mode != "DamageDealtOnce" && t.Mode != "AttackerBlocked" {
		return lRef{}, false, false, t, refuse("Effect triggers with RememberObjects$ %q", ro)
	}
	var ref lRef
	if lc.pairCap != nil {
		ref = *lc.pairCap // resolved once by the static-and-trigger pair
	} else if _, ok := ps.m["ValidTgts"]; !ok {
		var err error
		if ref, err = lc.parentCapture(); err != nil {
			return lRef{}, false, false, t, err
		}
	} else {
		var err error
		if ref, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return lRef{}, false, false, t, err
		}
	}
	if ref.Kind != rTarget {
		return lRef{}, false, false, t, refuse("Effect RememberObjects$ Targeted over %s", ref)
	}
	ps.consume("RememberObjects")
	return ref, true, false, t, nil
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
// DamageDone filter: the kernel matches the emblem's captured object on the
// side the flag was on, so the remaining filter is the type check. A
// ValidSource$ ...IsRemembered line captures the damage SOURCE (Emblem.Capture,
// the hook's srcObj); a ValidTarget$ ...IsRemembered line captures the damage
// RECIPIENT (Arcbond, Glyph of Life), and recipient reports that shape so the
// hook compares the damaged permanent instead. An AttackerBlocked one's
// ValidCard$ ...IsRemembered line is always a source capture. Any other
// Remembered read in the trigger refuses.
func emblemCaptureFilter(ab *lAbility) (bool, error) {
	t := ab.Trigger
	if t == nil || t.Event != "DamageDone" && t.Event != "DamageDealtOnce" && t.Event != "AttackerBlocked" {
		return false, refuse("Effect RememberObjects$ Targeted beside a trigger that is not ValidSource$ ...IsRemembered")
	}
	recipient := false
	if t.Event == "AttackerBlocked" {
		if t.F == nil || len(t.F.Clauses) != 1 || t.F.Players != 0 || t.F.Clauses[0].Flags&fRemembered == 0 {
			return false, refuse("Effect RememberObjects$ Targeted beside a trigger that is not ValidSource$ ...IsRemembered")
		}
		f := *t.F
		f.Clauses = append([]lClause(nil), f.Clauses...)
		f.Clauses[0].Flags &^= fRemembered
		t.F = &f
	} else {
		switch {
		case t.F != nil && len(t.F.Clauses) == 1 && t.F.Players == 0 && t.F.Clauses[0].Flags&fRemembered != 0:
			f := *t.F
			f.Clauses = append([]lClause(nil), f.Clauses...)
			f.Clauses[0].Flags &^= fRemembered
			t.F = &f
			if t.F2 != nil && t.F2.Players == 0 {
				// the hook keys the capture side on F2 being the object filter
				// (trig_effectuntileot.go embRecipientCapture), so this shape
				// (Acidic Dagger) must not lower: no corpus card lowers with
				// it, and the kernel cannot tell it from the recipient capture.
				return false, refuse("Effect RememberObjects$ Targeted over ValidSource$ ...IsRemembered beside an object ValidTarget$")
			}
		case t.F2 != nil && len(t.F2.Clauses) == 1 && t.F2.Players == 0 && t.F2.Clauses[0].Flags&fRemembered != 0:
			if t.F != nil {
				for i := range t.F.Clauses {
					if t.F.Clauses[i].Flags&fRemembered != 0 {
						// the source filter keeps a Remembered read the
						// kernel cannot match (the emblem's capture is
						// matched on the recipient side here): the emblem
						// would silently never fire
						return false, refuse("Effect RememberObjects$ Targeted beside a remembered ValidSource$ and a remembered object ValidTarget$")
					}
				}
			}
			f := *t.F2
			f.Clauses = append([]lClause(nil), f.Clauses...)
			f.Clauses[0].Flags &^= fRemembered
			t.F2 = &f
			recipient = true
		default:
			return false, refuse("Effect RememberObjects$ Targeted beside a trigger that is not ValidSource$ ...IsRemembered")
		}
	}
	// a DamageDealtOnce body may read the set its own links wrote (lowerCtx.capRem
	// made the first read the damage source); a read before any write is the
	// capture the kernel does not seed
	written := false
	for _, in := range ab.Prog {
		if kremReads(in.String()) && !written {
			return false, refuse("Effect trigger body reads Remembered beside RememberObjects$ Targeted")
		}
		written = written || t.Event == "DamageDealtOnce" && kremWrites(in)
	}
	return recipient, nil
}

// emblemDmgRecipientTrig reports whether tr is the captured emblem's
// recipient-captured DamageDone shape emblemCaptureFilter admits
// (ValidTarget$ ...IsRemembered on the damage RECIPIENT, no single-clause
// remembered ValidSource$ beside it): the kernel emblem hook then queues the
// DAMAGED object as the trigger object (dirac/trig_effectuntileot.go
// emblemTriggerObj over embRecipientCapture), and a body read of the damage
// source would name the damaged creature (lower_effect.go).
func emblemDmgRecipientTrig(tr *lTrigger) bool {
	if tr == nil || tr.Event != "DamageDone" {
		return false
	}
	if tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Players == 0 && tr.F.Clauses[0].Flags&fRemembered != 0 {
		return false // the source-capture shape; emblemCaptureFilter refuses it beside an object F2
	}
	return tr.F2 != nil && len(tr.F2.Clauses) == 1 && tr.F2.Players == 0 && tr.F2.Clauses[0].Flags&fRemembered != 0
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

// chosenCardCapture reads an until-end-of-turn Effect with no RememberObjects$
// whose DamageDone trigger's ValidSource$ is exactly Card.ChosenCardStrict
// (Azra Oddsmaker: "choose a creature. Whenever that creature deals combat
// damage to a player this turn"). gorge registers the Effect's trigger over
// the chosen creature as the resolution's chosen set stood at the Effect
// (effectChosenSourceTrigger), so the chain's later ClearChosenCard$ cannot
// erase it: the kernel's emblem captures that one object (rChosen, read when
// the emblem is created, before the cleanup's ClearChosen) and the filter is
// rewritten to the remembered spelling emblemCaptureFilter strips. The chain
// must hold exactly one ChooseCard before the Effect, so the chosen set is the
// one creature. Any other trigger is returned untouched with no capture.
func (lc *lowerCtx) chosenCardCapture(t cards.Trigger, eot bool) (lRef, bool, bool, cards.Trigger, error) {
	if t.Mode != "DamageDone" || strings.TrimSpace(t.Params["ValidSource"]) != "Card.ChosenCardStrict" {
		return lRef{}, false, false, t, nil
	}
	if !eot {
		return lRef{}, false, false, t, refuse("Effect trigger ValidSource$ Card.ChosenCardStrict beside a Duration$")
	}
	n := 0
	if lc.ab != nil {
		for _, in := range lc.ab.Prog {
			if in.Op == "ChooseCard" {
				n++
			}
		}
	}
	if n != 1 {
		return lRef{}, false, false, t, refuse("Effect trigger ValidSource$ Card.ChosenCardStrict after %d ChooseCard links", n)
	}
	cp := make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		cp[k] = v
	}
	cp["ValidSource"] = "Card.IsRemembered"
	t.Params = cp
	return lRef{Kind: rChosen}, true, false, t, nil
}

// capturedTriggerRef is the object a capturing emblem's trigger body reads as
// its remembered one: a DamageDealtOnce body's first Remembered is the damage
// source (lower_effectcaptured.go), and an AttackerBlocked body's RememberedLKI
// is the trigger object, the blocked attacker (the capture filter makes it the
// captured object).
func (lc *lowerCtx) capturedTriggerRef(v string) (lRef, bool) {
	if v == "Remembered" && lc.capRem && !lc.hasRememberWriter() {
		return lRef{Kind: rTriggeredSource}, true
	}
	if v == "RememberedLKI" && lc.emblemBlocked {
		return lRef{Kind: rTriggered}, true
	}
	return lRef{}, false
}
