package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// epicArmTailFor returns a fresh copy of an Epic spell's resolution chain with
// the arming instruction appended as its LAST sub-ability, or nil when the
// face does not print K:Epic.
//
// CR 702.50a grants both Epic clauses to a spell that RESOLVED ("after the
// spell resolves, ... for the rest of the game ... at the beginning of each
// of your upkeeps, copy this spell ..."). The arm therefore rides the spell's
// own resolution like Cipher's encode does (the same tail-shape precedent,
// engine/rules/cipher.go): no triggered stack object, no opponent priority
// window, and a countered, fizzled or effect-bounced Epic (which never
// reaches resolveTop's spell block) never arms. A ChangesZone arming trigger
// on the stack exit cannot do this: the resolution move, an effect-driven
// move and a counter move all carry empty Text on the event alike, so the
// empty-Text ResolvedOnly$ gate over-fires on a bounced spell.
//
// The chain is COPIED rather than mutated: the parsed card table is shared,
// immutable script data (the same rule the Cipher and GiftAbility splices
// follow). A shallow copy per node keeps the compiled-API cache intact while
// re-pointing Sub, so nothing downstream observes the original.
func epicArmTailFor(f *cards.Face, chain *cards.SA) *cards.SA {
	if f == nil || chain == nil || !f.HasKeywordRef(kwRefEpic) {
		return nil
	}
	arm := cards.ResolveSVar(f.SVars, "__kwEpicArm")
	if arm == nil {
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
	tail.Sub = arm
	return head
}

// spellTailSplices appends the tail instructions a spell's printed keywords
// arm as part of its OWN resolution, in their order, and is resolveTop's one
// splice site for that shape:
//
//   - Cipher (CR 702.99a, engine/rules/cipher.go's cipherTailFor): the encode
//     is the resolving spell's own tail instruction, not a triggered ability
//     -- a stack COPY (o.IsCopy) is not a card, so it cannot be encoded.
//   - Epic (CR 702.50a, epicArmTailFor above): the same tail shape arms the
//     can't-cast restriction and the first upkeep registration. Both clauses
//     belong to a spell that RESOLVED, so the tail -- which a countered,
//     fizzled or effect-bounced Epic never reaches -- is the arming vector,
//     not a stack-exit trigger. The same !o.IsCopy guard is the copy
//     exclusion: the upkeep body mints stack copies, and a copy's resolution
//     -- the copy is made except for its epic ability -- re-arms nothing.
func spellTailSplices(o *state.Object, f *cards.Face, resolveSA *cards.SA) *cards.SA {
	if o.IsCopy {
		return resolveSA
	}
	if withCipher := cipherTailFor(f, resolveSA); withCipher != nil {
		resolveSA = withCipher
	}
	if withEpic := epicArmTailFor(f, resolveSA); withEpic != nil {
		resolveSA = withEpic
	}
	return resolveSA
}
