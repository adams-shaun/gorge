package oracle

import "testing"

// Age of Ultron cast offers: p1 controls eight lands (>= six), so The World
// Tree's "lands you control have "{T}: Add one mana of any color"" static is
// live on every land, and p1's five untapped lands are five any-colour units
// of mana. Chase Stein, Runaway exiled Age of Ultron with its may-play
// permission, whose cast pays the printed cost (4{B}), so the may-play cast
// is payable and both engines must offer it (tail-08-c000 pair 54, decision
// 573, "options options": only the kernel offered it because gorge's
// potential planner could not price the payment).
func TestMBCAgeOfUltronReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "age_of_ultron_mayplay_census_gap")
}
