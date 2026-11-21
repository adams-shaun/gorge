package oracle

import "testing"

// Spinning Wheel Kick ({X}{X}{G}{G}) cast for X=0 with a Tower Worker tapped for {C}
// under Leyline of Abundance's "whenever you tap a creature for mana, add an
// additional {G}": the extra {G} and a Copperline Gorge's {G} pay {G}{G}. gorge
// offered the cast; the kernel did not (tail-21-c000 pair 28, decision 517).
func TestMBCSpinningWheelKickReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spinning_wheel_kick_x_zero_creature_tap_extra")
}
