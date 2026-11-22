package oracle

import "testing"

// CR 601.2g with CR 106.4: p0's only route to {4}{W} for Sentry Oak runs
// through a granted "N mana of any one colour" that the potential census
// widens into the source's tap unit. Priced as n independent any-colour mana
// the grant is splittable across the cast's white and colourless pips, so the
// kernel's bound pays for the cast; priced as one same-pick colour it is not,
// and the kernel under-offers the cast (only pass beside gorge's
// cast(Sentry Oak) at decision 692 of the recorded game).
func TestMBCSentryOakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_onecolour_widen")
}
