package oracle

import "testing"

// CR 601.2a/g and 603.5: Quest for the Gravelord's optional "you may put a
// quest counter on it" trigger is set off by the Eldrazi Scion sacrificed to
// fund Nevermaker's lowered evoke cast. Gorge's lowering activates the witness
// at priority before the cast, so the trigger goes on the stack and resolves
// (the lowering's default answer is "yes") before the spell is cast. The
// kernel left the optional trigger pending, so at decision 258 of pool
// dr-mbc-bbaa pair 1 the reference had quest counter 1 and one stack entry
// while the kernel had 0 and two.
func TestMBCFuriousAssaultReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "optional_trigger_before_lowered_cast")
}
