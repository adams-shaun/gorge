package deck

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// IsPartnerPair reports whether two cards may be a commander PAIR (CR
// 903.13): each carries a Partner-family ability and either both are plain
// Partners (whose "Friends forever" alias spells K:Partner:... and shares
// the head, CR 903.13a), or each "Partner with" the other by printed name
// (CR 903.13c). A plain Partner paired with a Partner-with card is not a
// legal pair (each half of a named pair names its own partner).
//
// This is the ONE partner-pair check: deck.ValidateCommander gates a
// two-commander deck file through it, and rules/engine.go's partnerPairOK
// (the engine's seating gate over rules.Config.Commanders) delegates here —
// the same arrangement as IsCommanderEligible, which the engine already
// delegates to. Validator and engine therefore cannot disagree about what a
// legal pair is.
func IsPartnerPair(a, b *cards.Card) bool {
	ha, hb := partnerHead(a), partnerHead(b)
	if ha == "" || hb == "" {
		return false
	}
	if ha == "Partner" && hb == "Partner" {
		return true
	}
	return partnerWithNames(a, b.Faces[0].Name) && partnerWithNames(b, a.Faces[0].Name)
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
