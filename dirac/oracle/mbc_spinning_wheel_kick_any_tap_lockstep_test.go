package oracle

import "testing"

// Spinning Wheel Kick ({X}{X}{G}{G}) at X=0 under Leyline of Abundance with a
// Deathbloom Gardener untapped and every land tapped: the Gardener's
// "{T}: Add one mana of any color" picked {G} plus Leyline's "whenever you tap
// a creature for mana, add an additional {G}" pays {G}{G}. gorge offered the
// cast; the kernel did not (tail-21-c000 pair 25, decision 602).
func TestMBCSpinningWheelKickAnyTapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spinning_wheel_kick_any_tap_extra")
}
