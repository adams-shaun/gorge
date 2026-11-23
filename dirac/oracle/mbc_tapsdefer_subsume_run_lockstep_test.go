package oracle

import "testing"

// CR 601.2g-h with CR 605.1a: with Manabarbs out every land p1 taps is a
// Taps-deferred gap, so seven lands (Islands, Swamps and a Blackcleave Cliffs)
// beside Aboshan's Desire's {U} spend gorge's plain and phased prefix searches'
// 164 nodes. gorge's third search, the subsume run, narrows each multi-answer
// colour ask (the Cliffs' {B} or {R}) to the colours the pool still lacks of
// the cost and finds the Islands' line, so it offers the cast; the kernel's
// subsets search had no such run and offered only pass (decision 543 of 543,
// tail-34-c000 pair 55).
func TestMBCAboshanSDesireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_subsume_run_offer")
}
