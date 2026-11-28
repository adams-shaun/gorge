package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A permanent that becomes a copy of a face-down permanent copies its
// face-down copiable values: a nameless 2/2 colourless Creature with no
// abilities and no mana cost (CR 707.2 "as modified by its face-down status",
// 707.3's example, 708.2), plus the copy's own exceptions (CR 707.9b). It is
// face up, so it cannot be turned face up (gorge builds the copy's face from
// the template's current face, state.Object.SetCopyFace).
//
// The copy layer keeps the template's card as ovr.copyOf, as every copy
// layer does, and rides the face-down derivation (face_facedown.go): the
// becomer takes gtyFaceDown, its types on ovGenTy and the walk flag, with
// both provenance bits set (gtyBlankCopy), a value no manifest or cloak
// writes, so blankCopy reads it as a copy and the turn-up, face-down filter
// and projection gates treat it as face up. The copied card is not
// subscribed: the blank values have no abilities. A face-down permanent that
// becomes a copy stays face down (CR 708.10) and is left alone.

// gtyBlankCopy is the provenance value of a permanent copy of a face-down
// permanent (face_manifest.go: manifested and cloaked are exclusive).
const gtyBlankCopy = gtyProvMask

// spareCopyFaceDown marks a token copy minted from a face-down permanent
// (copyTokenEnterFrom): the token keeps that permanent's provenance for
// faceDownWard, but it is itself face up (CR 707.2), so projFaceDown must not
// read the provenance as its own face-down status. markFaceDown clears it when
// the token later becomes face down in its own right.
const spareCopyFaceDown uint8 = 2

// copiedFaceDown reports a token copy minted from a face-down permanent.
func (s *State) copiedFaceDown(sl Slot) bool {
	return s.perms[sl].spare&spareCopyFaceDown != 0
}

// permBlank reports a permanent copy of a face-down permanent.
func (s *State) permBlank(sl Slot) bool {
	return s.perms[sl].gty&(gtyFaceDown|gtyProvMask) == gtyFaceDown|gtyBlankCopy
}

// copyBlankLayer settles the copy layer just put on the permanent at sl from
// the template at tmpl (NoSlot: a card, face up): blank when the template is
// face down, else the copied card's own values (a blank copy covered by a
// face-up copy drops its mark). It reports whether the layer is blank, so the
// caller subscribes no copied abilities.
//
//go:noinline
func (s *State) copyBlankLayer(k *prep, sl, tmpl Slot) bool {
	if k.gen == nil || !k.gen.anyFaceDown {
		return false
	}
	p := &s.perms[sl]
	blank := s.permBlank(sl)
	if p.gty&gtyFaceDown != 0 && !blank {
		return false // CR 708.10: a face-down permanent (or a blank token copy) keeps its face
	}
	if tmpl == NoSlot || !s.faceDown(k, tmpl) {
		if blank {
			s.clearFaceDown(k, sl)
		}
		return false
	}
	p.gty = kop.TCreature | gtyFaceDown | gtyBlankCopy | s.perms[tmpl].gty&gtyEffectDown // the template's ward, or none, is copiable
	p.ovr.flags |= ovGenTy
	p.ovr.walk = walkOn
	s.touchSl(sl)
	return true
}

// projFaceDown sets the projection flags of the face-down-valued permanent at
// sl: a permanently face-down one (a manifested/cloaked card, or a morph) is
// ProjFaceDown, a token copy of one is nameless, and a permanent copy of one
// keeps its printed card's name, as gorge projects it.
//
// The provenance marks come first: a permanent that is itself manifested or
// cloaked is face down (CR 708.2), while a copy OF a face-down permanent is
// face up (CR 707.2) even though it carries the source's provenance for its
// ward (copyTokenEnterFrom, faceDownWard). copiedFaceDown separates the two;
// blankCopy still catches the face-down-valued copies that are not marked,
// e.g. a copy of a morph.
func (s *State) projFaceDown(sl Slot, po *ProjObj) {
	switch {
	case (s.manifested(sl) || s.cloaked(sl)) && !s.copiedFaceDown(sl):
		po.Flags |= ProjFaceDown
	case !s.blankCopy(sl):
		po.Flags |= ProjFaceDown
	case !s.permBlank(sl):
		po.Flags2 |= ProjNameless
	}
}
