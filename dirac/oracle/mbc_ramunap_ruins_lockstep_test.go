package oracle

import (
	"testing"
)

// Ramunap Ruins's "{2}, {T}, Pay 1 life: Add {R}" activation is offered by
// gorge but not by the kernel: the priority decision at 286 of the pool's
// pair 131 (dr-mbc-abba, surface autopay) diverges with gorge offering
// activate(#33) and the kernel offering pass only.
func TestMBCRamunapRuinsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ramunap_ruins")
}
