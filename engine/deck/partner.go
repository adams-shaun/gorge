package deck

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// IsPartnerPair reports whether two cards may be a two-commander PAIR. Four
// pair families qualify, all from the same CR 903.13 family of abilities
// (the fourth is CR 702.124k):
//
//   - plain Partner (CR 903.13a): each card carries a Partner-family ability
//     and both are plain Partners — the "Friends forever" alias spells
//     K:Partner:... and shares the head;
//   - "Partner with" (CR 903.13c): each card names the other by printed name;
//   - Doctor's companion (the Doctor Who cycle): one card carries
//     K:Doctor's companion and the other is a Doctor (the creature subtype)
//     — or two distinct Doctors each carrying the companion keyword. The
//     companion's reminder text is "You can have two commanders if the other
//     is the Doctor.";
//   - Choose a Background (CR 702.124k): one card carries K:Choose a
//     Background and the other is a legendary Background enchantment.
//
// A plain Partner paired with a Partner-with card is not a legal pair (each
// half of a named pair names its own partner), and neither is a Doctor's
// companion paired with anything but a Doctor, nor a Choose-a-Background card
// paired with anything but a legendary Background.
//
// This is the ONE family check: IsCommanderPair (the full pair gate, adding
// each half's individual legality) is what deck.ValidateCommander and
// engine/rules' commanderPairOK call, and engine/rules/commander_config.go's
// partnerPairOK and the dirac adapter delegate here — the same arrangement as
// IsCommanderEligible. Validator and engine therefore cannot disagree about
// what a legal pair is, and a future pair family is added in exactly one place.
func IsPartnerPair(a, b *cards.Card) bool {
	if doctorCompanionPair(a, b) {
		return true
	}
	if isChooseABackgroundPair(a, b) {
		return true
	}
	ha, hb := partnerHead(a), partnerHead(b)
	if ha == "" || hb == "" {
		return false
	}
	if ha == "Partner" && hb == "Partner" {
		return true
	}
	return partnerWithNames(a, b.Faces[0].Name) && partnerWithNames(b, a.Faces[0].Name)
}

// IsCommanderPair reports whether a and b may be designated together as a
// seat's two commanders: they form a pair in one of the IsPartnerPair
// families AND each half is individually a legal commander, or is a legendary
// Background enchantment. The Background exception (CR 702.124k's last
// sentence: it can't be your commander unless paired with a Choose-a-
// Background card) is admitted only here, so a lone Background is still
// rejected by every single-commander path and IsCommanderEligible is not
// widened. Nil cards are never a pair.
func IsCommanderPair(a, b *cards.Card) bool {
	return IsPartnerPair(a, b) && commanderPairHalfLegal(a) && commanderPairHalfLegal(b)
}

// commanderPairHalfLegal reports whether c may be one half of a commander
// pair: individually commander-eligible, or a legendary Background enchantment.
func commanderPairHalfLegal(c *cards.Card) bool {
	return IsCommanderEligible(c) || isLegendaryBackgroundEnchantment(c)
}

// isChooseABackgroundPair reports whether a and b are a legal Choose-a-
// Background pair (CR 702.124k): one carries K:Choose a Background and the
// other is a legendary Background enchantment, in either order. Faces are read
// front-only like the Doctor family (CR 712.2).
func isChooseABackgroundPair(a, b *cards.Card) bool {
	return (hasChooseABackground(a) && isLegendaryBackgroundEnchantment(b)) ||
		(hasChooseABackground(b) && isLegendaryBackgroundEnchantment(a))
}

// hasChooseABackground reports whether c carries K:Choose a Background on its
// front face.
func hasChooseABackground(c *cards.Card) bool {
	return c != nil && len(c.Faces) > 0 && c.Faces[0].HasKeyword("Choose a Background")
}

// isLegendaryBackgroundEnchantment reports whether c's front face is a
// legendary enchantment with the Background subtype (case-insensitive over
// Types, the isDoctorCard read).
func isLegendaryBackgroundEnchantment(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	f := c.Faces[0]
	if !f.IsLegendary() || !f.IsEnchantment() {
		return false
	}
	for _, ty := range f.Types {
		if strings.EqualFold(strings.TrimSpace(ty), "Background") {
			return true
		}
	}
	return false
}

// doctorCompanionPair reports whether a and b are a legal Doctor's-companion
// pair. The clause reads "You can have two commanders if the other is the
// Doctor", so a legal pair is one where AT LEAST ONE half carries
// K:Doctor's companion and the OTHER half is a Doctor. That condition is
// directional and must hold in one direction or the other:
//
//   - the ordinary shape: exactly one companion half, and the other is a
//     Doctor;
//   - two distinct Doctors that EACH carry Doctor's companion: for each card
//     the other commander is the Doctor, so both directions hold and the pair
//     is legal. Requiring exactly one companion (ca == cb rejected) wrongly
//     denied this shape.
//
// It is NOT satisfied by two non-Doctor companions (neither half is a Doctor
// in either direction), nor by a companion paired with a non-Doctor.
//
// Both traits are read from the FRONT face only, like every other pair
// family here: the deck-construction rules consider a card outside the
// battlefield/stack by its front-face characteristics (CR 712.2 for
// transforming double-faced cards, CR 711.4 for modal ones), so a keyword or
// subtype printed only on the back face must not make a legal pair.
func doctorCompanionPair(a, b *cards.Card) bool {
	return (hasDoctorCompanion(a) && isDoctorCard(b)) ||
		(hasDoctorCompanion(b) && isDoctorCard(a))
}

// hasDoctorCompanion reports whether c carries the K:Doctor's companion
// keyword on its front face.
func hasDoctorCompanion(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	return c.Faces[0].HasKeyword("Doctor's companion")
}

// isDoctorCard reports whether c's front face carries the Doctor creature
// subtype (the "the Doctor" the companion clause names). The corpus prints it
// in the Types line ("Legendary Creature Time Lord Doctor"), so the match is
// case-insensitive over that face's Types.
func isDoctorCard(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	for _, ty := range c.Faces[0].Types {
		if strings.EqualFold(strings.TrimSpace(ty), "Doctor") {
			return true
		}
	}
	return false
}

// partnerHead reports the Partner-family head c carries, "" for none: the
// plain Partner ability, or "Partner with" (the CR 903.13c named pair).
func partnerHead(c *cards.Card) string {
	if c == nil || len(c.Faces) == 0 {
		return ""
	}
	for _, k := range c.Faces[0].Keywords {
		h := cards.KeywordHead(k)
		if h == "Partner" || h == "Partner with" {
			return h
		}
	}
	return ""
}

// partnerWithNames reports whether c carries a "Partner with" whose named
// partner is other (the corpus form is "Partner with:<name>[:<display>]";
// the first colon-field is the name).
func partnerWithNames(c *cards.Card, other string) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	for _, k := range c.Faces[0].Keywords {
		if cards.KeywordHead(k) != "Partner with" {
			continue
		}
		_, rest, ok := strings.Cut(k, ":")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, ":")
		if strings.EqualFold(strings.TrimSpace(name), other) {
			return true
		}
	}
	return false
}
