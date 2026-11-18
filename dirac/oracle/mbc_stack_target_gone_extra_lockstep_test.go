package oracle

import (
	"testing"
)

// CR 608.2b with the stack-ability target projection (tgt_stackability.go):
// Swarm Intelligence copies Katara's Reversal and the copy re-aims at the
// resolving Swarm Intelligence trigger itself (TargetType$
// Spell,Activated,Triggered) as its second counter target. Once that trigger
// leaves the stack, the copy still names it: gorge projects the vanished
// stack object as RefNone, so the kernel's projection must fold the copy's
// extra (second and later picks of one slot) stack-ability target to
// StackTargetGone too, not keep the stale StackTarget0+index code (which the
// gone entry's successor on the stack then answers for). Decision 733 of 733;
// the loop's own game (ev-tail-62-c001 pair 78, seed 1031301509820125960).
func TestMBCSwarmIntelligenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stack_target_gone_extra")
}
