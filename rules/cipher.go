package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
)

// Cipher's runtime encoded-card association and combat-damage trigger are
// implemented in trigger_granted.go; the printed expansion lives in cards.
func init() { effects.RegisterNonAPI("kw:Cipher") }

// cipherTailFor returns a fresh copy of a Cipher spell's resolution chain with
// the encode instruction appended as its LAST sub-ability, or nil when the
// face does not print K:Cipher.
//
// CR 702.99a makes the encode ("Then you may exile this spell card encoded on
// a creature you control") part of the spell's own resolution, so it runs
// after the spell's effects and before the resolved spell leaves the stack --
// no triggered stack object, no opponent priority window, and a countered or
// fizzled spell (which never reaches resolveTop's spell block) never encodes.
// Appending to the chain is what puts the ask under the existing suspension
// machinery: a mid-resolution ask parks the spell on the stack and the
// answered decision re-enters the same chain; a suspension in the spell's own
// body carries this tail in the continuation frames, so a resumed resolution
// reaches it exactly once.
//
// The chain is COPIED rather than mutated: the parsed card table is shared,
// immutable script data (the same rule the GiftAbility head splice in
// resolveTop follows). A shallow copy per node keeps the compiled-API cache
// intact while re-pointing Sub, so nothing downstream observes the original.
func cipherTailFor(f *cards.Face, chain *cards.SA) *cards.SA {
	if f == nil || chain == nil || !f.HasKeyword("Cipher") {
		return nil
	}
	cipher := cards.ResolveSVar(f.SVars, "__kwCipher")
	if cipher == nil {
		return nil
	}
	var copyChain func(sa *cards.SA) *cards.SA
	copyChain = func(sa *cards.SA) *cards.SA {
		if sa == nil {
			return nil
		}
		n := *sa
		n.Sub = copyChain(sa.Sub)
		return &n
	}
	head := copyChain(chain)
	tail := head
	for tail.Sub != nil {
		tail = tail.Sub
	}
	tail.Sub = cipher
	return head
}
