package oracle

import "testing"

// Arashin Sovereign ({5}{G}{W}) and Vagrant Plowbeasts, both seven mana, sat
// in the kernel's priority menu and not in gorge's with Lotus Blossom, Gift
// of Paradise on a Plains and a field of basics untapped (decision 502,
// surface autopay). The recording is on a custom pool, the table its own decks
// need, since the tail buckets re-cut whenever the lowered card set moves.
func TestMBCArashinSovereignReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lotus_blossom_petals_hold_back_seven_mana_cast")
}
