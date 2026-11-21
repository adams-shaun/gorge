package oracle

import "testing"

// CR 605.3b, 602.2b: Academy Ruins' "{1}{U}, {T}: Put target artifact card from
// your graveyard on top of your library" beside Starfleet Academy's "{5}, {T}:
// Add X mana of one color" (X the creature types among the player's creatures).
// The graveyard holds an artifact and the board's only {U} is that filter's.
// Gorge's planner never returns a Plan through a paid filter, so gorge does not
// offer the activation; the kernel offered it (decision 649 of 649; dr-mbc-abbb
// pair 24).
func TestMBCAcademyRuinsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paid_filter_activate_no_planner_witness")
}
