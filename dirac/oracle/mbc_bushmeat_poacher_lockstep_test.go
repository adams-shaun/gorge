package oracle

import "testing"

// Bushmeat Poacher's "{1}, {T}, Sacrifice another creature" activation: gorge
// offered activate(Bushmeat Poacher) at a priority decision where dirac offered
// only pass (decision 743 of tail-34-c000 pair 10).
func TestMBCBushmeatPoacherActivateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bushmeat_poacher_activate_offered")
}
