package rules

// Census of the trigger-side DamageAmount$ parameter.
// trigmatch.DamageMatchesWithCapture / DamageAmountMatches read it; before
// that nothing did, so Innocent Bystander investigated on Shock. The corpus
// carries it on exactly 8 trigger lines (plus 4 R: replacement lines, which
// replacementAmountMatches already reads). This ratchet fails when a new
// carrier appears on an unexpected mode, with an unknown operator, or with an
// operand the matcher cannot fold.

import (
	"sort"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// damageAmountUnreadableOperands are operands neither numeric nor an SVar on
// the carrying face: only Taii Wakeen's "EQTargetToughness" (damage equal to
// the recipient's toughness), which DamageAmountMatches reads from the damaged
// object (and fails closed on a player or a batched gate).
var damageAmountUnreadableOperands = map[string]bool{"TargetToughness": true}

func TestTriggerDamageAmountCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	modes := map[string]bool{"DamageDone": true, "DamageDoneOnce": true, "DamageDealtOnce": true, "DamageAll": true}
	ops := map[string]bool{"GE": true, "GT": true, "LE": true, "LT": true, "EQ": true}
	var carriers []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, tr := range f.Triggers {
				spec, ok := tr.Param(cards.PKDamageAmount)
				if !ok {
					continue
				}
				carriers = append(carriers, f.Name)
				if !modes[tr.Mode] {
					t.Errorf("%s: DamageAmount$ on unexpected mode %s", f.Name, tr.Mode)
				}
				if len(spec) < 3 || !ops[spec[:2]] {
					t.Errorf("%s: DamageAmount$ %q has no GE/GT/LE/LT/EQ operator", f.Name, spec)
					continue
				}
				rhs := spec[2:]
				if _, err := strconv.Atoi(rhs); err == nil {
					continue
				}
				if _, ok := f.SVars[rhs]; ok || damageAmountUnreadableOperands[rhs] {
					continue
				}
				t.Errorf("%s: DamageAmount$ operand %q is not numeric, an SVar, or a listed unreadable", f.Name, rhs)
			}
		}
	}
	sort.Strings(carriers)
	if len(carriers) != 8 {
		t.Errorf("trigger DamageAmount$ carriers = %d %v, want 8", len(carriers), carriers)
	}
}
