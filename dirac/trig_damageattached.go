package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// needsAttachedSource reports whether every alternative of damage trigger
// filter f reads the source Aura or Equipment's host (ValidSource$
// Card.AttachedBy: "whenever enchanted creature deals damage", Backfire).
// fireDamageSrc matches a source that is not a permanent -- a resolving
// spell, an ability whose source left -- by its card, and the card matcher
// cannot read an attachment, so such a source would match the filter
// vacuously; gorge's Card.AttachedBy is false of an object that is not
// attached by anything, so the line does not fire for it.
func needsAttachedSource(k *prep, f kop.FilterID) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		if k.t.Filters[f].Flags&kop.FAttachedBySrc == 0 {
			return false
		}
	}
	return true
}
