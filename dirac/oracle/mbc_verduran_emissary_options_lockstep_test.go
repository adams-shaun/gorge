package oracle

import "testing"

// Verduran Emissary's cast was offered twice by gorge (cast(#78,-,0) and
// cast(#78,-,1)) and once by dirac at decision 395 of the tail-26 pair 20
// game (seed 84395, mulligans 1, full surface), with Elven Bow and Colossal
// Badger in the same offer. The recording replays at main ec5795494d with
// that options divergence; a later main fixed it without a change here, so
// this is a regression guard only (it is green on current main).
func TestMBCVerduranEmissaryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "verduran_emissary_second_cast_option")
}
