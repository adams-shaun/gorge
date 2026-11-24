package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// api PreventDamage (CR 615.7; gorge effects/prevent_damage.go
// effPreventDamage) as OpExt ExtPreventNext: "prevent the next N damage that
// would be dealt to <recipients> this turn". The recipient is the ability's
// one target (any target, a creature, a player, a filtered creature), its
// Defined$ You / Self, the Aura's host (Defined$ Enchanted) or a set
// (Defined$ Valid ..., Radiance); N is Amount$ read once as the effect
// resolves. The shield depletes across damage events and lets the rest
// through (dirac/repl_prevnext.go).
//
// Several recipients lower as the matching shape: one shield per permanent
// matching a Defined$ Valid filter or a Radiance target's colour, and a
// divided total over a target slot (DividedAsYouChoose$) as one shield per
// chosen target with its announced share. A rider (PreventionSubAbility$)
// lowers to its application kind: the counter and delayed-counter bodies run
// at each application, and the retribution damage body deals the prevented
// amount to the rider's target (dirac/repl_prevnext.go, repl_prevx.go).
// Shapes the kernel has no record for stay refused, each with its true
// reason (the refusal class is still api:PreventDamage): a random recipient,
// a rider SVar the kernel does not run, and a set of recipients whose
// spelling is not the one loop shape.
func (lc *lowerCtx) preventNext(ps *params) error {
	rider, riderRef, _, err := lc.preventRider(ps)
	if err != nil {
		return err
	}
	combat, err := lc.preventCombat(ps)
	if err != nil {
		return err
	}
	if _, ok := ps.m["Random"]; ok {
		return refuse("api PreventDamage: Random: a random recipient")
	}
	if ps.is("Radiance", "True") {
		return lc.preventRadiance(ps, combat, rider, riderRef)
	}
	if d, ok := ps.get("Defined"); ok {
		d = strings.TrimSpace(d)
		switch {
		case d == "Enchanted":
			n, err := lc.amount(ps, "Amount", 0)
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "PreventNext", Obj: lRef{Kind: rEquipped}, A: n, C: combat | rider,
				W: riderRef, Note: "prevent the next A damage that would be dealt to the enchanted creature this turn"})
			return nil
		case strings.HasPrefix(d, "Valid "):
			return lc.preventDefinedValid(ps, strings.TrimSpace(strings.TrimPrefix(d, "Valid ")), combat, rider, riderRef)
		case d == "You" || d == "Self":
			// objOrTarget below reads it
		default:
			return refuse("api PreventDamage: Defined$ %q is a set of recipients", d)
		}
	}
	if _, ok := ps.m["DividedAsYouChoose"]; ok {
		return lc.preventDivided(ps, combat, rider, riderRef)
	}
	for _, k := range []struct{ key, why string }{
		{"TargetMax", "more than one recipient"},
		{"TargetMin", "a recipient count"},
	} {
		if _, ok := ps.m[k.key]; ok {
			return refuse("api PreventDamage: %s: %s", k.key, k.why)
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
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, A: n, C: combat | rider, W: riderRef,
		Note: "prevent the next A damage that would be dealt to Obj this turn"})
	return nil
}

// preventCombat reads CombatDamage$: True only (a combat-damage shield,
// Decorated Griffin). C 1 is the kernel's combat-only flag.
func (lc *lowerCtx) preventCombat(ps *params) (int, error) {
	if v, ok := ps.get("CombatDamage"); ok {
		if strings.TrimSpace(v) != "True" {
			return 0, refuse("api PreventDamage: CombatDamage$ %q", v)
		}
		return int(kop.PrevNextCombat), nil
	}
	return 0, nil
}

// preventDivided lowers DividedAsYouChoose$: the target slot declares the
// division (TargetMin$/TargetMax$ over the divided total), and the kernel
// installs one shield per chosen target with the share announced for it
// (dirac/repl_prevnext.go). A value total (Pollen Remedy's kicked 6 or 3,
// Serra's Hymn's verse counters) is TargetSpec.Divide == DivValue, read where
// the ability is put on the stack (CR 601.2d).
func (lc *lowerCtx) preventDivided(ps *params, combat, rider int, riderRef lRef) error {
	div, _ := ps.get("DividedAsYouChoose")
	dv, err := lc.value(div)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Amount", 0)
	if err != nil {
		return err
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("api PreventDamage: DividedAsYouChoose$ over recipient %s", obj)
	}
	t := &lc.ab.Targets[obj.N]
	if dv.isConst() {
		t.Divided = dv.N
		t.DividedV = lValue{}
	} else {
		if mn := t.Min; !mn.isConst() || mn.N != 0 {
			// gorge aborts a cast whose total is below the slot's minimum; the
			// kernel's clamp assumes "any number"
			return refuse("api PreventDamage: divided over a value total with a TargetMin$ above 0")
		}
		t.Divided = kop.DivValue
		t.DividedV = dv
	}
	lc.emit(lInstr{Op: "PreventNext", A: n, C: combat | rider | int(kop.PrevNextDivided), W: lRef{Kind: rTarget, N: obj.N},
		Note: "prevent the next A damage divided among the chosen targets"})
	return nil
}

// preventRadiance lowers Radiance$ True (Wojek Apothecary): the target
// creature and each other creature sharing a colour with it each get a
// shield (gorge's Radiance expansion). The kernel reads the target's colours
// and the battlefield at resolution (dirac/repl_prevnext.go).
func (lc *lowerCtx) preventRadiance(ps *params, combat, rider int, riderRef lRef) error {
	n, err := lc.amount(ps, "Amount", 0)
	if err != nil {
		return err
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return refuse("api PreventDamage: Radiance without ValidTgts$")
	}
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("api PreventDamage: Radiance target filter %q names players", vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield})
	lc.emit(lInstr{Op: "PreventNext", Obj: lRef{Kind: rTarget, N: t}, A: n,
		C: combat | rider | int(kop.PrevNextRadiance), W: riderRef,
		Note: "prevent the next A damage to the target and each creature sharing a colour with it"})
	return nil
}

// preventDefinedValid lowers Defined$ Valid <filter> (Kitsune Palliator,
// Samite Censer-Bearer): one shield on every matching permanent, and, when
// the filter names players too, one on every player. The kernel loops the
// battlefield with the filter (and the living players) and installs a shield
// on each (dirac/repl_prevnext.go).
func (lc *lowerCtx) preventDefinedValid(ps *params, spec string, combat, rider int, riderRef lRef) error {
	n, err := lc.amount(ps, "Amount", 0)
	if err != nil {
		return err
	}
	// Forge writes a union with "&" (Kitsune Palliator's "Creature & Player");
	// the kernel loops the object half with a filter and the player half as a
	// player set, so lower each arm on its own.
	for _, part := range strings.Split(spec, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := lc.filter(part)
		if err != nil {
			return err
		}
		objF := *f
		if f.Players != 0 {
			if f.Players != plBoth {
				return refuse("api PreventDamage: Defined$ Valid %q names a player subset", spec)
			}
			objF.Players = 0
			if len(objF.Clauses) != 0 {
				lc.emit(lInstr{Op: "PreventNext", A: n, C: combat | rider | int(kop.PrevNextMultiObj),
					F: &objF, W: riderRef, Note: "prevent the next A damage to each matching permanent"})
			}
			lc.emit(lInstr{Op: "PreventNext", A: n, C: combat | rider | int(kop.PrevNextMultiPlayer),
				W: riderRef, Note: "prevent the next A damage to each player"})
			continue
		}
		lc.emit(lInstr{Op: "PreventNext", A: n, C: combat | rider | int(kop.PrevNextMultiObj),
			F: &objF, W: riderRef, Note: "prevent the next A damage to each matching permanent"})
	}
	return nil
}

// preventRider reads PreventionSubAbility$/ShieldEffectTarget$: the rider
// body runs once per application (gorge replacement_damage.go's
// runPreventionShieldRider). The lowered kinds (kop.PrevNextRider*):
//
//   - PutCounter$ P1P1 with CounterNum$ PreventedDamage: put that many +1/+1
//     counters on the rider's target at each application (Test of Faith,
//     Temper).
//   - the delayed Effect: its one End of Turn Phase trigger's Execute$ is a
//     PutCounter$ P0P1 with CounterNum$ PreventedDamage; the prevented
//     amount accumulates and is put at the beginning of the next end step
//     (Sacred Boon, Scars of the Veteran).
//   - DealDamage$ with NumDmg$ PreventedDamage: deal that much damage to the
//     rider's target at each application (Acolyte's Reward, Vengeful Archon).
//
// The rider's target is the shield's own recipient (ShieldEffectTarget$
// Targeted), the parent ability's chosen target (ParentTarget) or the
// controller (You). Any other body, scope or SVar keeps the refusal.
func (lc *lowerCtx) preventRider(ps *params) (int, lRef, bool, error) {
	name := ps.str("PreventionSubAbility")
	scope := ps.str("ShieldEffectTarget")
	if name == "" {
		if _, ok := ps.m["ShieldEffectTarget"]; ok {
			return 0, lRef{}, false, refuse("api PreventDamage: ShieldEffectTarget$ %q without PreventionSubAbility$", scope)
		}
		return 0, lRef{}, false, nil
	}
	sub := cards.ResolveSVar(lc.face.SVars, name)
	if sub == nil {
		return 0, lRef{}, false, refuse("api PreventDamage: PreventionSubAbility$ %q resolves to no ability", name)
	}
	rider := 0
	gate := false
	switch sub.API {
	case "PutCounter":
		if !strings.EqualFold(strings.TrimSpace(sub.ParamStr(cards.PKCounterNum)), "PreventedDamage") {
			return 0, lRef{}, false, refuse("api PreventDamage: rider CounterNum$ %q", sub.ParamStr(cards.PKCounterNum))
		}
		switch strings.TrimSpace(sub.ParamStr(cards.PKCounterType)) {
		case "P1P1":
			rider = int(kop.PrevNextRiderCounter)
		default:
			return 0, lRef{}, false, refuse("api PreventDamage: rider CounterType$ %q", sub.ParamStr(cards.PKCounterType))
		}
	case "DealDamage":
		if !strings.EqualFold(strings.TrimSpace(sub.ParamStr(cards.PKNumDmg)), "PreventedDamage") {
			return 0, lRef{}, false, refuse("api PreventDamage: rider NumDmg$ %q", sub.ParamStr(cards.PKNumDmg))
		}
		rider = int(kop.PrevNextRiderDamage)
	case "Effect":
		g, err := lc.preventDelayedCounterRider(sub)
		if err != nil {
			return 0, lRef{}, false, err
		}
		rider, gate = int(kop.PrevNextRiderDelayedCounter), g
	default:
		return 0, lRef{}, false, refuse("api PreventDamage: PreventionSubAbility$ %q is a %s body", name, sub.API)
	}
	var ref lRef
	switch scope {
	case "Targeted":
		// the shield's own recipient
	case "ParentTarget":
		t := lc.lastTarget()
		if t < 0 {
			return 0, lRef{}, false, refuse("api PreventDamage: rider ShieldEffectTarget$ ParentTarget without a parent target")
		}
		ref = lRef{Kind: rTarget, N: t}
	case "You":
		ref = lRef{Kind: rYou}
	default:
		return 0, lRef{}, false, refuse("api PreventDamage: rider ShieldEffectTarget$ %q", scope)
	}
	if gate {
		// the delayed body's IsPresent$ Creature.IsRemembered gate: mark the
		// record with a ref the kernel reads as "still a creature" (the
		// delayed counter rider names no target of its own)
		if scope != "Targeted" {
			return 0, lRef{}, false, refuse("api PreventDamage: creature-gated delayed rider with ShieldEffectTarget$ %q", scope)
		}
		ref = lRef{Kind: rSelf}
	}
	return rider, ref, gate, nil
}

// preventDelayedCounterRider accepts exactly the delayed counter Effect rider
// shape gorge runs (cards.PreventionRiderDelayedCounterEffect): one Phase
// trigger at End of Turn whose Execute$ is a PutCounter$ P0P1 of
// CounterNum$ PreventedDamage. Scars of the Veteran's IsPresent$ gate is read
// at fire time by the kernel.
func (lc *lowerCtx) preventDelayedCounterRider(sub *cards.SA) (bool, error) {
	names := strings.Fields(sub.ParamStr(cards.PKTriggers))
	if len(names) != 1 {
		return false, refuse("api PreventDamage: delayed rider with %d Triggers$", len(names))
	}
	tr, ok := cards.ParseTriggerLine(lc.face.SVars[names[0]])
	if !ok || strings.TrimSpace(tr.Mode) != "Phase" || !strings.EqualFold(strings.TrimSpace(tr.ParamStr(cards.PKPhase)), "End of Turn") {
		return false, refuse("api PreventDamage: delayed rider trigger %q is not an end-of-turn Phase trigger", names[0])
	}
	present := strings.TrimSpace(tr.ParamStr(cards.PKIsPresent))
	if present != "" && !strings.EqualFold(present, "Creature.IsRemembered") {
		return false, refuse("api PreventDamage: delayed rider IsPresent$ %q", present)
	}
	body := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(tr.ParamStr(cards.PKExecute)))
	if body == nil || body.API != "PutCounter" ||
		!strings.EqualFold(strings.TrimSpace(body.ParamStr(cards.PKCounterNum)), "PreventedDamage") ||
		strings.TrimSpace(body.ParamStr(cards.PKCounterType)) != "P0P1" {
		return false, refuse("api PreventDamage: delayed rider body is not a P0P1 PreventedDamage PutCounter")
	}
	return present != "", nil
}
