package oracle

import "testing"

// CR 605.3a with CR 702.51a and CR 601.2b: Apprentice Sharpshooter and Wall
// of Vines convoked a Chord of Calling ({X}{G}{G}{G}) the payer's window
// prices past a Desolate Mire ({1}, {T}: Add {W}{B}, a first-only filter):
// the window funds the {1} fee with mana floated earlier in the same window,
// so its reach is the twelve plain units plus the Mire's net one, and gorge
// xAsk offered X up to 12. The kernel's X-ask bound (filtMaxX's walk) once
// priced the raw cost there, ignoring the announced picks, so its bound
// stopped at 10 and the ask offered one X fewer (tail-75 pair 7, decision
// 1001, options).
func TestMBCApprenticeSharpshooterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filtfee_x_bound_convoke_picks")
}
