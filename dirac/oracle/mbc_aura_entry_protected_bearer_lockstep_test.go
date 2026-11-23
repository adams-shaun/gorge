package oracle

import "testing"

// CR 303.4f, 704.5m: Nomad Mythmaker returns Krovikan Whispers attached to a
// creature the Whispers' own entry makes protected (Empty-Shrine Kannushi has
// protection from the colors of permanents its controller controls). The
// entry's one attach choice is spent; the Aura enters unattached and the
// state-based action sends it back. gorge once asked the choice a second time
// after the refused attach (decision 367, tail-06 pair 28).
func TestMBCKrovikanWhispersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_entry_protected_bearer_asks_once")
}
