package oracle

import "testing"

// CR 106.6, 605.1a, 602.2b: Sage of the Unknowable's {C} may be spent only on a
// colorless spell or to activate an ability, and paying the {1} of Ferrous
// Lake's "{1}, {T}: Add {U}{R}" activates an ability, so it funds that filter
// land, whose {U} then pays Shinen of Flight's Wings' channel cost ({U},
// discard it). gorge offers the channel activation from the hand with
// Sage and the Lake as the only untapped sources; the kernel did not.
func TestMBCShinenOfFlightSWingsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_mana_pays_filter_land_cost")
}
