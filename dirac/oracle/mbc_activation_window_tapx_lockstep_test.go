package oracle

import "testing"

// CR 605.3a, 601.2g-h: gorge's manual window for an activation lists a
// tap-another-permanent mana source (Springleaf Drum) in battlefield order, and
// the harness answer takes the first listed. In std-ecl pair 26 (evl-11190431)
// Figure of Fable's {1}{G/W}{G/W} paid its W pip through that window: gorge
// tapped Springleaf Drum (and Figure of Fable for it), the kernel left the
// source out of an activation's window unless a sacrifice-another source was
// live and tapped the Plains after it. The activation window now lists it as
// the cast window does (castTapX).
func TestActivationWindowListsTapAnotherSourceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "activation_window_lists_tap_another_source")
}
