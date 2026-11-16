package oracle

import "testing"

// Agent of Stromgald's "{R}: Add {B}" is a repeatable fee-only filter. With two
// Mountains and a Swamp beside it, gorge's potential walk (PotentialMana counts
// the filter's {B} once, so two) never lists Geralf's Messenger's {B}{B}{B}, and
// the prefix search runs only on that walk: the kernel's any-mana candidate
// walk offered the cast (decision 96, tail-08 pair 21, mulligans 0, seed 1).
func TestMBCGeralfSMessengerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "repeatable_filter_mana_pips")
}
