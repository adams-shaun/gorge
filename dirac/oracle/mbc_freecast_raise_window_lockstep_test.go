package oracle

import "testing"

// CR 601.2f with CR 601.2g-h: p0 played Lay Claim without paying its mana
// cost (Urza, Lord High Artificer's "{5}: Shuffle your library, then exile the
// top card. Until end of turn, you may play that card without paying its mana
// cost"), and p1's Kasmina, Enigmatic Mentor taxes the free cast {2} when it
// targets a creature or planeswalker she controls. The plain sources fall
// short of the {2} (one untapped Mountain), so the target menu asks the CR
// 601.2g window, whose tap-another production (Urza's "Tap an untapped
// artifact you control: Add {U}") reaches it: every target of hers stays on
// the menu, beside the ones the raise never prices. The kernel's free-cast
// arm priced the {2} against the plain sources alone and dropped her every
// creature and planeswalker (decision 760 of 760; dr-mbc-baaa pair 3, seed
// 741061).
func TestMBCKasminaEnigmaticMentorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "freecast_raise_window")
}
