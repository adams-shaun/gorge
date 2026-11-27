package kgen

import "strings"

// entryFaceDown reads ChangeZone's FaceDown$ True on a move to the
// battlefield: the card enters face down (CR 708.2, kop.RetFaceDown) rather
// than through the manifest procedure. FaceDownSetType$ (Yedora's "It's a
// Forest land") stays unlowered: a face-down basic land's intrinsic mana
// ability is not modelled (the kernel hides every ability of a face-down
// permanent, and Yedora's type set makes the returned card a Forest), so the
// card is refused by name and tracked in a follow-up ticket.
// FaceDownPower$/FaceDownToughness$ are the wider set-type family and stay
// refused; no card in this change carries them.
func (lc *lowerCtx) entryFaceDown(ps *params, dest int, in *lInstr) error {
	fd, hasFD := ps.get("FaceDown")
	if _, hasSet := ps.get("FaceDownSetType"); hasSet {
		return refuse("FaceDownSetType$ (the face-down type set stays unlowered)")
	}
	if !hasFD {
		return nil
	}
	if strings.TrimSpace(fd) != "True" {
		return refuse("FaceDown$ %q", fd)
	}
	if dest != zBattlefield {
		return refuse("FaceDown$ to %s", zoneLabel[dest])
	}
	if _, ok := ps.get("StaticEffect"); ok {
		return refuse("FaceDown$ beside StaticEffect$ (the enter-with rider is not applied by the face-down entry)")
	}
	for _, k := range []string{"FaceDownPower", "FaceDownToughness"} {
		if _, ok := ps.get(k); ok {
			return refuse("FaceDown$ with %s (the face-down P/T set stays unlowered)", k)
		}
	}
	ps.consume("FaceDown")
	in.CS = strings.TrimPrefix(in.CS+"|FaceDown", "|")
	return nil
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
