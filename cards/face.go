package cards

import (
	"strconv"
	"strings"
)

func (f *Face) hasType(t string) bool {
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

func (f *Face) IsLand() bool         { return f.hasType("Land") }
func (f *Face) IsCreature() bool     { return f.hasType("Creature") }
func (f *Face) IsInstant() bool      { return f.hasType("Instant") }
func (f *Face) IsSorcery() bool      { return f.hasType("Sorcery") }
func (f *Face) IsArtifact() bool     { return f.hasType("Artifact") }
func (f *Face) IsEnchantment() bool  { return f.hasType("Enchantment") }
func (f *Face) IsPlaneswalker() bool { return f.hasType("Planeswalker") }

// IsPermanent reports whether resolving this face puts it onto the battlefield.
func (f *Face) IsPermanent() bool { return !f.IsInstant() && !f.IsSorcery() }

// KeywordHead strips a keyword's parameters: "Equip:2" is the Equip keyword.
// Keywords whose name contains spaces ("Protection from blue") keep them.
func KeywordHead(k string) string {
	if i := strings.IndexByte(k, ':'); i >= 0 {
		k = k[:i]
	}
	return strings.TrimSpace(k)
}

func (f *Face) HasKeyword(k string) bool {
	for _, x := range f.Keywords {
		if strings.EqualFold(KeywordHead(x), k) {
			return true
		}
	}
	return false
}

// KeywordParam returns the text after the colon of a parameterised keyword
// ("Kicker:B" -> "B"; "Equip:2" -> "2") and reports whether the keyword is
// printed at all ("Flash" -> "", true; absent -> "", false).
func (f *Face) KeywordParam(head string) (string, bool) {
	for _, k := range f.Keywords {
		if strings.EqualFold(KeywordHead(k), head) {
			if i := strings.IndexByte(k, ':'); i >= 0 {
				return strings.TrimSpace(k[i+1:]), true
			}
			return "", true
		}
	}
	return "", false
}

// SpellAbility is the SP$ ability a card casts with, if any.
func (f *Face) SpellAbility() *SA {
	for _, a := range f.Abilities {
		if a.Kind == "SP" {
			return a
		}
	}
	return nil
}

// ManaAbilities lists every activated ability that produces mana.
func (f *Face) ManaAbilities() []*SA {
	var out []*SA
	for _, a := range f.Abilities {
		if a.Kind == "AB" && a.API == "Mana" {
			out = append(out, a)
		}
	}
	return out
}

func (f *Face) pt(i int) int {
	parts := strings.SplitN(f.PT, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
	if err != nil {
		return 0 // "*" and other characteristic-defining values; layer 7a owns these
	}
	return n
}

func (f *Face) Power() int     { return f.pt(0) }
func (f *Face) Toughness() int { return f.pt(1) }
