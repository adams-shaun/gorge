package oracle

import "testing"

// Network Terminal's {1}, {T}, tap another untapped artifact: the bot's first
// pick (Ojutai Monument, the only spare mana source) strands the {1}, so the
// activation is reversed before any mana ability (CR 733.1); a second such
// reversal with no progress holds the option out (CR 733.2).
func TestMBCNetworkTerminalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapx_activation_held_out")
}
