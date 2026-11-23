package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// FwSameName: the sameName context bases (T13b-2 remnants). Forge's
// Remembered./Targeted./Triggered. base prefixes carry sameName's referent
// (gorge sameNameContextBase, engine/effects/filter_value.go): the candidate
// shares a printed name with the object the base names, resolved by
// sameNameContextReferent against the evaluating context — the entry's first
// remembered object, its first live object target, or its triggering card.
// The name identity is fwSharesName's, and so is the word's body; only the
// referents it is read with differ (the sameName bases name ONE object, the
// share words' referents may name several).

// fwSameName is kop.FwSameName: the candidate shares a printed name with THE
// object the referent names — the base resolves one referent (gorge
// sameNameContextReferent returns the first live one and binds only that),
// so unlike the share words the read is the resolved set's first object,
// never any of them. Unbound referent: false (gorge's fail-closed
// sameNameContextReferent skips the alternative).
//
//kmanifest:fword FwSameName
func (s *State) fwSameName(c *fctx, l *kop.FLit) bool {
	if c.fd {
		return false
	}
	cn := NoCard
	if c.perm() {
		cn = s.nameCardAt(c.sl)
	} else if c.c != NoCard {
		cn = s.nameKey(c.c)
	}
	if cn == NoCard {
		return false
	}
	refs := s.fwReferentObjs(c, l.R)
	if len(refs) == 0 {
		return false
	}
	rn := s.objName(refs[0])
	return rn != NoCard && rn == cn
}
