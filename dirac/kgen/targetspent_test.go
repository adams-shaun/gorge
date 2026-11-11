package kgen

import (
	"strings"
	"testing"
)

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

// TestTargetedCastSpentAdmission: the snapshot reads target slot 0 at
// resolution start, so only a resolving spell whose one slot is slot 0 and
// was declared at cast may lower Targeted$CastTotalManaSpent. A modal or
// at-resolution slot would read the wrong slot (or nothing) while the
// kernel's snapTgtSpent reads e.tgt[0].
func TestTargetedCastSpentAdmission(t *testing.T) {
	spell := lTarget{Spell: true, Zone: zStack, Min: cst(1), Max: cst(1)}
	atRes := spell
	atRes.AtRes = true
	for _, tc := range []struct {
		name  string
		ab    *lAbility
		chain []int
		want  string
	}{
		{"the only slot is slot 0", &lAbility{Kind: akSpell, Targets: []lTarget{spell}}, []int{0}, ""},
		{"two target slots", &lAbility{Kind: akSpell, Targets: []lTarget{spell, spell}}, []int{0, 1}, "over 2 target slots"},
		{"the spell target is slot 1", &lAbility{Kind: akSpell, Targets: []lTarget{{}, spell}}, []int{1}, "not the snapshot's slot 0"},
		{"the target is chosen at resolution", &lAbility{Kind: akSpell, Targets: []lTarget{atRes}}, []int{0}, "chosen at resolution"},
		{"outside a resolving spell", &lAbility{Kind: akActivated, Targets: []lTarget{spell}}, []int{0}, "outside a resolving spell"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lc := &lowerCtx{ab: tc.ab, chainT: tc.chain}
			v, err := lc.targetedCastSpentValue()
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error %v, want %q", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if v.Kind != vTgtSpent {
				t.Fatalf("kind %d, want vTgtSpent", v.Kind)
			}
		})
	}
}
