package rules

import "github.com/adams-shaun/gorge/engine/state"

// combat_damage_view.go: what the view needs of combatRound to tell dirac's two
// damage steps apart (view.DamageStepReader). Reads only.

// Names of the damage step a priority round follows (view.View.DamageStep).
const (
	damageStepFirstStrike = "first-strike"
	damageStepRegular     = "regular"
)

// damageStepName is the damage step the priority round at step st follows: the
// first-strike pass's round, then the regular pass's round, "" while a pass is
// still dealing or asking for a division, and off the combat damage step.
func damageStepName(r *combatRound, st state.Step) string {
	switch {
	case st != state.StepCombatDamage:
		return ""
	case r.regularDone:
		return damageStepRegular
	case r.hasFirst && r.firstDone && !r.firstEnded && !r.active:
		return damageStepFirstStrike
	}
	return ""
}

// CombatDamageStep reports which priority round of the combat damage step the
// game is in ("first-strike" or "regular"), "" outside one.
func (e *Engine) CombatDamageStep() string { return damageStepName(&e.combatRound, e.G.Step) }

// FirstStrikePass reports whether the combat damage step runs a first-strike
// pass (combatRound.hasFirst); false off the step.
func (e *Engine) FirstStrikePass() bool {
	return e.G.Step == state.StepCombatDamage && e.combatRound.hasFirst
}
