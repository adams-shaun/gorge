package rules

// combat_damage_view.go: what the view needs of combatRound to tell dirac's two
// damage steps apart (view.DamageStepReader). Reads only.
//
// It is the one *Engine method this change adds, paid for by dropping
// dealDamagePass, a one-line wrapper of damageStep: the Engine method count and
// surface are shrink-only ratchets.

// Names of the damage step a priority round follows (view.View.DamageStep).
const (
	damageStepFirstStrike = "first-strike"
	damageStepRegular     = "regular"
)

// CombatDamageStep reports which priority round of the combat damage step the
// game is in ("first-strike", after the first-strike pass, or "regular", after
// the regular pass; "" while a pass is dealing or asking for a division) and
// whether the step runs a first-strike pass (combatRound.hasFirst). combatRound
// is zero off the combat damage step, so both are empty there too.
func (e *Engine) CombatDamageStep() (round string, firstStrikePass bool) {
	r := &e.combatRound
	switch {
	case r.regularDone:
		round = damageStepRegular
	case r.hasFirst && r.firstDone && !r.firstEnded && !r.active:
		round = damageStepFirstStrike
	}
	return round, r.hasFirst
}
