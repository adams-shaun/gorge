package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A damage trigger whose ValidSource$ is Card.AttachedBy (kop.FAttachedBySrc:
// Vampiric Link and Spirit Link's "whenever enchanted creature deals damage",
// Curiosity's "deals damage to an opponent") watches the permanent its Aura or
// Equipment is attached to. The card matcher fireDamageSrc reads has no
// battlefield to look at and skips the flag, so every spell matched it:
// Tendrils of Corruption killing a creature fired a Vampiric Link sitting on
// Typhoid Rats (dr-mbc-abaa pair 42). A source that is no permanent is
// nobody's host (CR 303.4, 301.5; gorge attachedBy wants both on the
// battlefield), so a source filter made only of host alternatives admits none.

// hostOnlySrc reports whether every alternative of filter f is a host
// alternative (FAttachedBySrc): such a filter admits no source off the
// battlefield. A mixed chain stays with the card matcher.
func hostOnlySrc(k *prep, f kop.FilterID) bool {
	if f == 0 {
		return false
	}
	for ; f != 0; f = k.t.Filters[f].Or {
		if k.t.Filters[f].Flags&kop.FAttachedBySrc == 0 {
			return false
		}
	}
	return true
}
