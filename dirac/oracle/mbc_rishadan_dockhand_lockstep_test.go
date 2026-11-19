package oracle

import "testing"

// A may-play cast whose mana can only be funded by a counter-cost source is a
// prefix-scripted play in gorge's planner, and the kernel must offer it too:
// Siphon Insight's exiled Rishadan Dockhand ({U}) sits in p1's exile under the
// may-play-any-colour grant, and p0's only untapped source is Mirrodin's Core
// holding a charge counter. Gorge's may-play plan could fund the folded pip
// but not a coloured one (the planner defers a source's SubCounter
// activation), so its census/prefix fallback never ran and the cast was not
// offered; the kernel, which scripts the counter activation, offered it.
// Recorded as tail-07 pair 29 (surface autopay).
func TestMBCRishadanDockhandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rishadan_dockhand")
}
