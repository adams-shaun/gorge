package oracle

import (
	"testing"
)

// Forge's bare CheckSVar$/ConditionCheckSVar$ gate (no SVarCompare$ /
// ConditionSVarCompare$) reads its SpellAbilityVariables field defaults:
// sVarOperator "GE" over sVarOperand "1", i.e. the gate holds at one or more,
// never at any nonzero value (StaticAbility's getParamOrDefault("SVarCompare",
// "GE1") reads the same). gorge's CheckSVarCompare once read a bare gate as
// plain "nonzero", so Tithe's optional second search -- "If target opponent
// controls more lands than you, you may search for an additional Plains
// card", whose gate SVar is target lands minus yours -- was offered when the
// opponent controlled FEWER lands (X < 0): gorge asked its optional-search
// confirm while dirac (whose lowering compiles the bare gate to GE1) had
// already resolved the spell, and the lockstep parted at the spell's zone
// (decision 1273 of the recorded game: projection Cards[79].Zone). The
// recorded game is a tail-01-c000 evidence-loop pair whose owed evidence was
// filed as Ichor Slick; the measured carrier of this class is Tithe.
func TestMBCIchorSlickReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "checksvar_no_compare_defaults_ge1")
}
