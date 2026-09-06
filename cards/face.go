package cards

import (
	"strconv"
	"strings"
)

// manaBraceForm normalises a brace-form mana cost ("{2}{U}{U}") to the
// space-separated form cmcFromManaCost parses. Built once at package scope,
// not per call, because derive runs on every face of the corpus at load.
var manaBraceForm = strings.NewReplacer("{", " ", "}", " ")

func (f *Face) hasType(t string) bool {
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

func (f *Face) IsLand() bool         { return f.hasType("Land") }
func (f *Face) IsBasic() bool        { return f.hasType("Basic") }
func (f *Face) IsLegendary() bool    { return f.hasType("Legendary") }
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

func (f *Face) Power() int     { return int(f.power) }
func (f *Face) Toughness() int { return int(f.toughness) }

// Cmc returns the face's converted mana cost, derived once at load from the
// printed ManaCost string. It mirrors botpolicy.CmcOf's arithmetic exactly
// (cards cannot import botpolicy or rules, so the few lines are duplicated
// here by design) so a face read the same way anywhere agrees. {X} counts as
// 0 off the stack, a hybrid/Phyrexian/colourless symbol as one generic.
func (f *Face) Cmc() int32 { return f.cmc }

// CharacteristicDefining reports whether the face's printed P/T is a
// characteristic-defining value ("*", "1+*"): Power()/Toughness() return 0
// for these and layer 7a (in rules) supplies the real value.
func (f *Face) CharacteristicDefining() bool { return f.characteristicDefining }

// Colour identity is a bitmask over the five colours packed into one byte. A
// bitmask is the natural representation: identity is used as a set-membership
// question ("commander identity must be a superset of this card's identity")
// and a subset test is a single bitwise-and versus zero, it is order
// independent (so scanning fields in any order can never couple identity to
// iteration order, which determinism forbids elsewhere), and one byte is the
// smallest thing a Commander subset check needs. The identity itself is the
// CR 903.5 notion, not the card's colour: a card with no mana cost can still
// have an identity, and a colourless artifact whose activated ability costs
// {R} is red-identity.
const (
	ColourWhite uint8 = 1 << iota
	ColourBlue
	ColourBlack
	ColourRed
	ColourGreen
)

// ColourIdentity returns the face's colour identity, derived once at load
// from its mana cost, colour indicator (Colors:) and the mana symbols in its
// rules text. See deriveColourIdentity for exactly which fields contribute.
func (f *Face) ColourIdentity() uint8 { return f.colourIdentity }

// derive computes the derived fields from the printed text fields. It must
// run after every path that constructs a Face values its printed fields from
// text — after ParseBytes and after the gob decode path — so the two
// construction routes produce identical faces. It is never run into the gob:
// the derived fields stay unexported (gob ignores them) and are recomputed on
// decode, so a stale cache whose gob zero-filled them is repaired with no
// error anywhere.
func (f *Face) derive() {
	f.power, f.toughness, f.characteristicDefining = parsePT(f.PT)
	f.cmc = cmcFromManaCost(f.ManaCost)
	f.colourIdentity = f.deriveColourIdentity()
}

// deriveColourIdentity computes the face's colour identity from its printed
// text. It is the CR 903.5 definition: every colour in the mana cost, plus
// the colour indicator (Forge's Colors:, present only on the cards whose
// colour a viewer cannot otherwise infer — Dryad Arbor is green but casts for
// no cost), plus every colour-contributing mana symbol anywhere in its rules
// text — an activated ability's Cost$ (Ghoulcaller Gisa's {B}), a Produced$
// value, a trigger, a static or a replacement (Charm/Thopterist-class cards
// whose identity lives in an ability, not the cost). Reminder text is not
// scanned at all: it lives in Oracle, which is deliberately excluded, so the
// overarching "({T}: Add {R}.)" of a basic land contributes nothing (a basic
// land has empty identity, by design).
//
// The fields that carry rules text are: ManaCost, Colors, every ability's
// raw Line (which includes its Cost$/Produced$/SpellDescription$ and
// SubAbility references), the params of every trigger/static/replacement, and
// every SVar value. Not scanned, deliberately: Keywords (a Kicker:{X} style
// optional cost is not a mana cost or a rules-text pip and following it would
// invent identity from optional extra costs) and Oracle (reminder text).
func (f *Face) deriveColourIdentity() uint8 {
	var m uint8
	m |= manaColours(f.ManaCost)   // the printed mana cost
	m |= colourIndicator(f.Colors) // the colour indicator
	for _, a := range f.Abilities {
		// Intrinsic granted basic-land mana abilities are not printed rules
		// text (a Mountain has no "{T}: Add {R}" in its oracle) and must not
		// contribute identity; they are also added after derive on the
		// ParseBytes route but present before it on the gob route, so skipping
		// them keeps the two construction routes identical.
		if strings.HasPrefix(a.Line, "intrinsic:") {
			continue
		}
		m |= manaColours(a.Line)
	}
	for _, t := range f.Triggers {
		m |= manaColours(t.Mode)
		for _, v := range t.Params {
			m |= manaColours(v)
		}
	}
	for _, s := range f.Statics {
		for _, v := range s.Params {
			m |= manaColours(v)
		}
	}
	for _, r := range f.Repls {
		for _, v := range r.Params {
			m |= manaColours(v)
		}
	}
	for _, v := range f.SVars {
		m |= manaColours(v)
	}
	return m
}

// colourIndicator maps a Colors: value ("black", "white,blue", "colorless")
// to its colour bitmask. Colourless contributes nothing; unknown names are
// ignored.
func colourIndicator(s string) uint8 {
	var m uint8
	for _, part := range strings.Split(s, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "white":
			m |= ColourWhite
		case "blue":
			m |= ColourBlue
		case "black":
			m |= ColourBlack
		case "red":
			m |= ColourRed
		case "green":
			m |= ColourGreen
		}
	}
	return m
}

// isManaCh reports whether c can form part of a mana symbol as Forge writes
// them: the colour letters, C/S colourless and snow, P Phyrexian, T tap, a
// slash (hybrid "2/B", "W/U") and the digits of a generic cost. Because
// lowercase letters are never part of a written pip, matching only these lets
// a prose word keep its uppercase
// initial without it being read as a colour.
func isManaCh(c byte) bool {
	switch c {
	case 'W', 'U', 'B', 'R', 'G', 'C', 'P', 'S', 'T', '/',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

func isAlphaNum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// manaColours scans s for colour-contributing mana symbols and returns their
// bitmask. A symbol is a run of isManaCh characters bounded on both sides by
// a non-alphanumeric, so neither the tail of a word nor an uppercase initial
// inside one ("White", "Swamp", the "CARD" in CARDNAME) is misread as a pip.
// Only the WUBRG letters contribute: C (colourless), S (snow), P (Phyrexian),
// numerics and slashes never add a colour, matching CR. Hybrid (WU, W/U) and
// Phyrexian (WP, UP, RP) forms contribute their colour letters and nothing
// else. The same scanner runs over the ManaCost and every rules-text field so
// a pip counts wherever it appears — in the cost, in a Cost$, in a Produced$,
// in a trigger, in an SVar.
func manaColours(s string) uint8 {
	var m uint8
	n := len(s)
	i := 0
	for i < n {
		// Advance to a run start: a mana char whose left neighbour is not
		// alphanumeric, so it cannot be the tail of a longer word.
		for i < n && !(isManaCh(s[i]) && (i == 0 || !isAlphaNum(s[i-1]))) {
			i++
		}
		if i >= n {
			break
		}
		var cm uint8
		j := i
		for j < n && isManaCh(s[j]) {
			switch s[j] {
			case 'W':
				cm |= ColourWhite
			case 'U':
				cm |= ColourBlue
			case 'B':
				cm |= ColourBlack
			case 'R':
				cm |= ColourRed
			case 'G':
				cm |= ColourGreen
			}
			j++
		}
		// The run must also be a whole token, not the head of a longer prose
		// word ("White"): require a non-alphanumeric (or end) right after it.
		if j >= n || !isAlphaNum(s[j]) {
			m |= cm
		}
		i = j
	}
	return m
}

// parsePT splits a printed P/T ("2/2") into power and toughness. A face with
// no P/T yields 0,0 and flag false; a face whose P/T carries a
// characteristic-defining value ("*", "1+*") yields 0 for the affected side
// (exactly what pt used to return) and sets the flag, because layer 7a owns
// that value.
func parsePT(pt string) (pow, tgh int32, cd bool) {
	parts := strings.SplitN(pt, "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	for i, s := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			cd = true
			continue // 0 for this side; layer 7a owns characteristic-defining values
		}
		if i == 0 {
			pow = int32(n)
		} else {
			tgh = int32(n)
		}
	}
	return pow, tgh, cd
}

// cmcFromManaCost is cards' own conversion of a printed ManaCost string to a
// converted mana cost, an exact mirror of botpolicy.CmcOf. It deliberately
// re-derives rules/mana.go's ParseCost.CMC() by hand here because cards can
// import neither botpolicy nor rules.
func cmcFromManaCost(mc string) int32 {
	mc = manaBraceForm.Replace(mc)
	mc = strings.TrimSpace(mc)
	if mc == "" || strings.EqualFold(mc, "no cost") {
		return 0
	}
	var n int32
	for _, sym := range strings.Fields(mc) {
		if sym == "X" { // {X} is 0 off the stack
			continue
		}
		if len(sym) == 1 && strings.ContainsRune("WUBRGC", rune(sym[0])) { // a single coloured/colourless pip
			n++
			continue
		}
		if v, err := strconv.Atoi(sym); err == nil && v >= 0 {
			n += int32(v)
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbol: one generic.
		n++
	}
	return n
}
