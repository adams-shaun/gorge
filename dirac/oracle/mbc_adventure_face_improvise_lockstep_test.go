package oracle

import "testing"

// CR 702.126a, 715.3a: with Inspiring Statuary out, Amethyst Dragon's
// Adventure (Explosive Crystal, {4}{R}) is a nonartifact spell with improvise,
// but gorge's pool-only offer of a face prices the raw face cost, and its
// potential plan declines a credited face ("unsupported", "cost:credit"). The
// floating pool alone does not pay {4}{R}, so only dirac offered the castface,
// which its pool test paid with the improvise credit (decision 209 of 209;
// tail-08 pair 755).
func TestMBCAmethystDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "adventure_face_improvise_grant")
}
