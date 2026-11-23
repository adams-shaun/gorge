package oracle

import "testing"

// Feeling of Dread ("tap up to two target creatures") cast on a board where
// its controller's every land is tapped and the only mana left is a floating
// {W} beside Vodalian Arcanist's restricted {C} ("spend this mana only to
// cast an instant or sorcery spell"): the empty (zero-target) answer costs
// the plain {1}{W} — Charix's "spells your opponents cast that target
// Charix cost {2} more" applies only to a selection that names Charix — and
// {1}{W} is payable from the floating pool, the restricted {C} paying the
// generic. The target ask therefore offers done (CR 601.2c before 601.2f).
// The kernel's emptyTargetSetPayable read the plain pool alone, withheld
// done, and the game diverged (pool tail-34 pair 3, seed 288584, decision
// 474).
func TestMBCFeelingOfDreadReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "empty_target_set_ignores_restricted_pool")
}
