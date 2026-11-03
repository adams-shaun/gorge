package kgen

import "reflect"

// eotCombatEmblemRefuse refuses the until-end-of-turn Effect triggers on
// combat events the kernel's emblem hooks do not carry exactly
// (dirac/trig_effectuntileot.go; gorge effEffect registers the body as a
// recurring delayed trigger, trigger_delayed.go delayedEventMatches). A
// DamageDone emblem fires with amount 0 (the kernel's emblem enqueue), so a
// body reading the damage amount refuses; its ValidTarget$ is exactly
// Player (a mixed player-and-object target falls to build's second-filter
// refusal). An AttackersDeclared one counts every attacker of the
// declaration.
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
	if condReadsAmt(ab.Trigger.Cond) || condReadsAmt(ab.ActCond) {
		return refuse("Effect DamageDone trigger reads the damage amount")
	}
	for i := range ab.Prog {
		in := &ab.Prog[i]
		if valReadsAmt(in.A) || valReadsAmt(in.B) || condReadsAmt(in.Cond) {
			return refuse("Effect DamageDone trigger reads the damage amount")
		}
	}
	return nil
}

func valReadsAmt(v lValue) bool {
	if v.Kind == vTriggerAmount {
		return true
	}
	for _, s := range v.Sub {
		if valReadsAmt(s) {
			return true
		}
	}
	return false
}

func condReadsAmt(c lCond) bool {
	if valReadsAmt(c.A) || valReadsAmt(c.B) {
		return true
	}
	for _, s := range c.Sub {
		if condReadsAmt(s) {
			return true
		}
	}
	return false
}
