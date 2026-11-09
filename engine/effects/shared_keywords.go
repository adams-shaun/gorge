package effects

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Forge's SharedKeywordsZone$/SharedRestrictions$ filter, the one reading of a
// "a keyword is shared if some card in these zones has it" grant. Pump and
// PumpAll consume it while resolving (sharedPumpGrant); rules' printed
// Mode$ Continuous static scan consumes it while building the layer-6 grant
// (Eater of Virtue, Cairn Wanderer, Soulflayer).

// ParseSharedKeywordZones parses a SharedKeywordsZone$ comma list. A word
// this build cannot classify fails the whole list (false), the fail-closed
// direction: a grant must not widen to a zone the script did not name.
func ParseSharedKeywordZones(raw string) ([]state.Zone, bool) {
	var zones []state.Zone
	for part := range strings.SplitSeq(raw, ",") {
		z, ok := ParseZoneWord(part)
		if !ok {
			return nil, false
		}
		zones = append(zones, z)
	}
	return zones, len(zones) > 0
}

// ParseSharedKeywordRestrictions parses SharedRestrictions$, a comma list of
// independent card filters. An absent value is Forge's default, Card.
func ParseSharedKeywordRestrictions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"Card"}
	}
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// SharedKeywordSpellings keeps every spelling of a wanted keyword that a
// sharing card has. A family word in want (Landwalk, Protection, Hexproof,
// Trample) stands for every parameterized spelling of that keyword, and the
// sharing card's exact spelling is what is returned: a card with "Protection
// from red" shares that exact protection, never a bare "Protection". Every
// other wanted word matches a sharing keyword case-insensitively by its full
// spelling.
func SharedKeywordSpellings(want, have []string) []string {
	var out []string
	add := func(kw string) {
		if !slices.Contains(out, kw) {
			out = append(out, kw)
		}
	}
	for _, w := range want {
		family := sharedKeywordFamilyWord(w)
		for _, h := range have {
			switch {
			case family != "":
				if strings.EqualFold(sharedKeywordFamily(h), family) {
					add(h)
				}
			case strings.EqualFold(h, w):
				add(w)
			}
		}
	}
	return out
}

// sharedKeywordFamilyWord reports the bare family word a WANTED keyword stands
// for, or the empty string when the keyword must match by its full spelling.
func sharedKeywordFamilyWord(kw string) string {
	switch kw {
	case "Landwalk", "Protection", "Hexproof", "Trample":
		return kw
	}
	return ""
}

// sharedKeywordFamily is the family word a sharing card's keyword spelling
// belongs to: "Protection from red" and "Protection:Card.Red:..." are both
// Protection. The empty string means the keyword is its own spelling (Flying,
// Lifelink).
func sharedKeywordFamily(kw string) string {
	if strings.HasPrefix(kw, "Protection") {
		return "Protection"
	}
	switch head := cards.KeywordHead(kw); head {
	case "Landwalk", "Hexproof", "Trample":
		return head
	}
	return ""
}
