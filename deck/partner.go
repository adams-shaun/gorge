package deck

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// IsPartnerPair reports whether two cards may be a two-commander PAIR. Three
// pair families qualify, all from the same CR 903.13 family of abilities:
//
//   - plain Partner (CR 903.13a): each card carries a Partner-family ability
//     and both are plain Partners — the "Friends forever" alias spells
//     K:Partner:... and shares the head;
//   - "Partner with" (CR 903.13c): each card names the other by printed name;
//   - Doctor's companion (the Doctor Who cycle): exactly one card carries
//     K:Doctor's companion and the other is a Doctor (the creature subtype).
//     The companion's reminder text is "You can have two commanders if the
//     other is the Doctor."
//
// A plain Partner paired with a Partner-with card is not a legal pair (each
// half of a named pair names its own partner), and neither is a Doctor's
// companion paired with anything but a Doctor.
//
// This is the ONE pair check for every family: deck.ValidateCommander gates
// a two-commander deck file through it, and rules/engine.go's partnerPairOK
// (the engine's seating gate over rules.Config.Commanders) delegates here —
// the same arrangement as IsCommanderEligible, which the engine already
// delegates to. Validator and engine therefore cannot disagree about what a
// legal pair is, and a future pair family is added in exactly one place.
func IsPartnerPair(a, b *cards.Card) bool {
	if doctorCompanionPair(a, b) {
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

// doctorCompanionPair reports whether a and b are a legal Doctor's-companion
// pair: exactly one carries K:Doctor's companion and the other is a Doctor.
// The subtype match is case-insensitive over every face's Types, the same
// read the rest of the deck package uses for a subtype.
func doctorCompanionPair(a, b *cards.Card) bool {
	ca, cb := hasDoctorCompanion(a), hasDoctorCompanion(b)
	// Exactly one half must carry the companion keyword: two companions have
	// no Doctor between them, and a pair of two Doctors carries no companion
	// clause at all (the printed ability always names the companion half).
	if ca == cb {
		return false
	}
	if ca {
		return isDoctorCard(b)
	}
	return isDoctorCard(a)
}

// hasDoctorCompanion reports whether c carries the K:Doctor's companion
// keyword on any face.
func hasDoctorCompanion(c *cards.Card) bool {
	if c == nil {
		return false
	}
	for i := range c.Faces {
		if c.Faces[i].HasKeyword("Doctor's companion") {
			return true
		}
	}
	return false
}

// isDoctorCard reports whether any face of c carries the Doctor creature
// subtype (the "the Doctor" the companion clause names). The corpus prints it
// in the Types line ("Legendary Creature Time Lord Doctor").
func isDoctorCard(c *cards.Card) bool {
	if c == nil {
		return false
	}
	for i := range c.Faces {
		for _, ty := range c.Faces[i].Types {
			if strings.EqualFold(strings.TrimSpace(ty), "Doctor") {
				return true
			}
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
