package oracle

import "testing"

// Snake Basket's "{X}, Sacrifice Snake Basket: Create X 1/1 green Snake
// creature tokens" activation asks X (CR 601.2b). gorge offered choose:x 0..8,
// the kernel 0..10 (decision 823 of tail-27 pair 27, surface full, seed
// 663985): an Eldrazi Scion taps for Battery Bearer's granted "{T}: Add {C}"
// and then sacrifices itself for its printed "{Sacrifice this: Add {C}", so
// each Scion makes two mana and X = 10 is reachable (CR 602.2b/605.1a). The
// cast window counted each Scion once.
func TestMBCSnakeBasketReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "snake_basket_x_reach")
}
