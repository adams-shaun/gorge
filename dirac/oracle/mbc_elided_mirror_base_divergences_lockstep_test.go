package oracle

import "testing"

// A game the kernel's own re-derivation of gorge's payment plan diverges on,
// and the elided-mana mirror (elided.go) cleans: recorded as divergent on the
// build without the mirror (tail-49, seed 496348, a kfuzz scan of 100 pairs)
// and replayed clean with it. It is the mirror's own pin: a recording that is
// clean without it (the elided_mirror_* games the mirror's bugs were recorded
// from) cannot fail when the mirror is removed.
//
//   - elided_mirror_plan_sources_match_gorges_applied_plan: the kernel's
//     re-derivation tapped other sources than gorge's plan did (two permanents'
//     flags, 2 life, two pool units apart) before the cast.
func TestMBCElidedMirrorBaseDivergencesReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"elided_mirror_plan_sources_match_gorges_applied_plan",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamedServed(t, name) })
	}
}
