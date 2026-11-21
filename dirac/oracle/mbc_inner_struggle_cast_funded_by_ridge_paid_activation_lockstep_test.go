package oracle

import "testing"

// CR 601.2g with CR 106.6: at decision 174 of synth tail-73 pair 2
// (seed 11690), p0 controls untapped Mountain, Mountain, Raging Ravine and
// Shadowblood Ridge. Inner Struggle costs {3}{R}: Raging Ravine's {T} pays the
// {R}, two Mountains and Shadowblood Ridge's R pay the three generics, and the
// {1} of Shadowblood Ridge's own activation ({1}, {T}: Add {B}{R}) is paid
// first from a Mountain's {R}. Its {B} then pays a generic. Untaidake's {C}{C}
// cannot help: spend-this-mana-only-to-cast-legendary-spells bans it from a
// nonlegendary spell (CR 106.6). The cast is therefore a legal priority
// option, and the gorge side offered it while the kernel withheld it when its
// restricted-mana accounting over-restricted the sources (the kgen
// restrictedmana filter words and their revert bracket the window). The
// recording replays clean today, pinning the kernel's offer: a regression
// back to the withholding fails this replay's option comparison.
func TestMBCInnerStruggleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "inner_struggle_cast_funded_by_ridge_paid_activation")
}
