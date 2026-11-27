package kgen

import "strings"

// entryFaceDown reads ChangeZone's FaceDown$ True on a move to the
// battlefield: the card enters face down (CR 708.2, kop.RetFaceDown) rather
// than through the manifest procedure. FaceDownSetType$ folds the card
// text's face-down type set (Yedora's "It's a Forest land") into an override
// record applied as the permanent enters and only while it is face down
// (kop.Override.FaceDownOnly). FaceDownPower$/FaceDownToughness$ are the
// wider set-type family and stay refused; no brief card carries them.
func (lc *lowerCtx) entryFaceDown(ps *params, dest int, in *lInstr) (*lOverride, error) {
	fd, hasFD := ps.get("FaceDown")
	setType, hasSet := ps.get("FaceDownSetType")
	if !hasFD {
		if hasSet {
			return nil, refuse("FaceDownSetType$ without FaceDown$")
		}
		return nil, nil
	}
	if strings.TrimSpace(fd) != "True" {
		return nil, refuse("FaceDown$ %q", fd)
	}
	if dest != zBattlefield {
		return nil, refuse("FaceDown$ to %s", zoneLabel[dest])
	}
	if _, ok := ps.get("StaticEffect"); ok {
		return nil, refuse("FaceDown$ beside StaticEffect$ (the enter-with rider is not applied by the face-down entry)")
	}
	for _, k := range []string{"FaceDownPower", "FaceDownToughness"} {
		if _, ok := ps.get(k); ok {
			return nil, refuse("FaceDownSetType$ with %s (the P/T set type stays unlowered)", k)
		}
	}
	ps.consume("FaceDown")
	in.CS = strings.TrimPrefix(in.CS+"|FaceDown", "|")
	if !hasSet {
		return nil, nil
	}
	ps.consume("FaceDownSetType")
	o := &lOverride{FaceDownOnly: true}
	for _, w := range strings.Split(setType, "&") {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if b, ok := typeAddBit(w); ok {
			o.SetTypes |= b
			o.HasSetTypes = true
			continue
		}
		if !lc.known(w) {
			return nil, refuse("FaceDownSetType$ word %q", w)
		}
		o.SetSubtypes = append(o.SetSubtypes, w)
		o.HasSetSubtypes = true
	}
	if !o.HasSetTypes && !o.HasSetSubtypes {
		return nil, refuse("FaceDownSetType$ %q has no type", setType)
	}
	return o, nil
}

// digSeenFaceDownLook reports whether a Dig that exiles face down without
// WithMayLook$ reads as the may-look layout: the Dig window was seen
// (NoLooking$ beside ExileFaceDown$ refuses in digNoLooking), and gorge's
// effDig seenFaceDownExile rewrites the blind marker to the may-look layout,
// which is the kernel's DigFaceDown (Ugin, the Ineffable). A blind exile, a
// may-look one and any other destination keep look as read.
func digSeenFaceDownLook(fd, look, blind bool, dest int) bool {
	return look || (fd && !blind && dest == zExile)
}
