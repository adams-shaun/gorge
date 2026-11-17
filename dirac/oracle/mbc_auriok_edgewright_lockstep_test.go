package oracle

import "testing"

// CR 106.4, 605.3b: Lotus Blossom's "{T}, Sacrifice Lotus Blossom: Add X mana
// of any one color, where X is the number of petal counters on it" funds a
// cast. The kernel's cast census left every dynamic {T}, sacrifice source out,
// so the priority menu below the recorded game omitted Auriok Edgewright's and
// Orthodoxy Enforcer's casts (decision 158, tail-74 pair 5).
func TestMBCAuriokEdgewrightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dynamic_sac_mana_pays_cast")
}
