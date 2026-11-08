package oracle

import "testing"

// CR 608.2h: Tarrian's Soulcleaver's "put a +1/+1 counter on equipped
// creature" resolves after the Equipment and its bearer have both left the
// battlefield. gorge's Object.LastBearer names the bearer the Equipment was
// most recently attached to (Akawalli), never an earlier bearer from a
// previous departure (Pugnacious Hammerskull), so no counter lands on the
// creature that is still on the battlefield.
func TestEquippedLastBearerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-lci", []stdRepro{
		{"equipped_lastbearer_after_host_departure", true, false},
	})
}
