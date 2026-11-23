package oracle

import "testing"

// Arcbond's emblem remembers its target creature and fires when that creature
// is dealt damage, whatever the damage's source. Cave-In (a spell, no
// permanent) damaged the remembered Saga creature, so gorge queued the
// emblem's trigger beside the two Devil tokens' dies triggers and asked
// player 1 to order all three; the kernel fired emblems for permanent sources
// only and offered two (decision 600 of tail-37 pair 140).
func TestMBCArcbondReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "emblem_recipient_capture_spell_source_damage")
}
