package oracle

import "testing"

// Snake Basket's "{X}, Sacrifice Snake Basket: Create X 1/1 green Snake
// creature tokens" activation asks X (CR 601.2b). gorge offers choose:x 0..8,
// the kernel 0..10: the payment planner's reach over the board differs
// (decision 823 of tail-27 pair 27, surface full, seed 663985).
func TestMBCSnakeBasketReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "snake_basket_x_reach")
}
