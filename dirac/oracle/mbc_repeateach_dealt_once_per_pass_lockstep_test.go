package oracle

import "testing"

// CR 120.x with gorge's RepeatEach DamageMap$ True (engine/effects/
// repeateach.go): the loop's damage is one damage batch, so a DamageDealtOnce
// line latches per dealing source over the whole loop (trigger_match.go
// damageBatchKey). Rakdos Charm's "Each creature deals 1 damage to its
// controller" binds each creature exactly once, so the line queues once, at
// its source's own pass, with that pass's amount -- the kernel's per-pass
// fire is the same game, and a line that never matches (Descendant of
// Kiyomaro's "Whenever this creature deals combat damage", DamageDealtOnce |
// CombatDamage$ True, beside the loop's non-combat damage) queues nothing.
// The kernel used to stop instead (StallLowering at act_damagefrom.go),
// pairing tail-36-c000 pair 77.
func TestMBCTail36ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "repeateach_dealt_once_per_pass")
}
