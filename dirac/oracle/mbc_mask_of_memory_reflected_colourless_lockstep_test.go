package oracle

import "testing"

// CR 601.2h/605.1a and CR 107.4c: Mask of Memory costs {2}, and seat 0's only
// untapped mana is A-Faceless Haven ({T}: Add {C}) plus Horizon of Progress
// ("{T}, Pay 1 life: Add one mana of any type that a land you control could
// produce" -- Horizon itself reflects A-Faceless Haven's {C}). The payment
// planner dropped a reflected ability's colourless production, so gorge did
// not offer the cast dirac offered (decision 61, kfuzz synth tail-71 focus
// pair 63).
func TestMBCMaskOfMemoryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mask_of_memory_reflected_colourless")
}
