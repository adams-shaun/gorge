package kgen

import "testing"

// TestTargetSpentForms: a resolving spell reads its target spell's cast
// spend (Targeted$CastTotalManaSpent) as ExTgtSpent, snapshotted at
// resolution start, and its mana value (Targeted$CardManaCost) as the
// existing ExMVOf over the target slot. Unravel counters the spell, then
// draws only when the spend was less than the countered card's mana value.
func TestTargetSpentForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Unravel", form: "Targeted$CastTotalManaSpent + Targeted$CardManaCost after a Counter", lir: "0 Counter obj=Target0\n     1 SkipIf if=Value(op5,TgtSpent,MVOf(Target0)) skip=1\n     2 Draw obj=You A=1"},
	})
}
