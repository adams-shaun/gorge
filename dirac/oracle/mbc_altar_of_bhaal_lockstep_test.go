package oracle

import "testing"

// Altar of Bhaal ({2}{B}, {T}, exile a creature you control: return a creature
// card from your graveyard to the battlefield, sorcery speed): dirac once
// offered activate(#0{Altar of Bhaal}) at a priority decision (906 of
// tail-61-c000 pair 9) where gorge did not. The recording is that game's
// 906-action prefix; a fix since main c623d13a5b closed it.
func TestMBCAltarOfBhaalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "altar_of_bhaal_priority_activate")
}
