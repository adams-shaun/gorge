package oracle

import (
	"testing"
)

// Graveyard Shovel's "{2}, {T}: Target player exiles a card from their
// graveyard" is offered at priority by both engines (decision 531 of 531,
// dr-tail-29 pair 2).
func TestMBCGraveyardShovelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "graveyard_shovel_activation_offered")
}
